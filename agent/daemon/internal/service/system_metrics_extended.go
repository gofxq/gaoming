package service

import (
	"context"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/sensors"
)

type collectorResult struct {
	name     string
	points   []contracts.MetricPoint
	failures int
	duration time.Duration
	timedOut bool
	apply    func(*monitorv1.SystemMetrics)
}

func collectExtendedMetrics(ctx context.Context) ([]contracts.MetricPoint, []contracts.CollectorStatus, []string, *monitorv1.SystemMetrics) {
	collectors := []struct {
		name string
		run  func(context.Context) collectorResult
	}{
		{name: "host", run: collectHostMetrics},
		{name: "cpu", run: collectCPUMetrics},
		{name: "memory", run: collectMemoryMetrics},
		{name: "swap", run: collectSwapMetrics},
		{name: "load", run: collectLoadMetrics},
		{name: "filesystem", run: collectFilesystemMetrics},
		{name: "disk_io", run: collectDiskIOMetrics},
		{name: "network", run: collectNetworkMetrics},
		{name: "sensors", run: collectSensorMetrics},
	}
	resultCh := make(chan collectorResult, len(collectors))
	for _, collector := range collectors {
		go func() {
			startedAt := time.Now()
			result := collector.run(ctx)
			result.duration = time.Since(startedAt)
			result.timedOut = ctx.Err() == context.DeadlineExceeded
			resultCh <- result
		}()
	}

	byName := make(map[string]collectorResult, len(collectors))
	for len(byName) < len(collectors) {
		select {
		case result := <-resultCh:
			byName[result.name] = result
		case <-ctx.Done():
			goto collected
		}
	}

collected:

	var points []contracts.MetricPoint
	statuses := make([]contracts.CollectorStatus, 0, len(collectors))
	capabilities := make([]string, 0, len(collectors))
	structured := &monitorv1.SystemMetrics{}
	for _, collector := range collectors {
		result, completed := byName[collector.name]
		state := contracts.CollectorStateAvailable
		if !completed || len(result.points) == 0 {
			state = contracts.CollectorStateUnavailable
		} else if result.failures > 0 {
			state = contracts.CollectorStateDegraded
		}

		metricNames := uniqueMetricNames(result.points)
		statuses = append(statuses, contracts.CollectorStatus{
			Name:           collector.name,
			State:          state,
			MetricNames:    metricNames,
			DurationMicros: result.duration.Microseconds(),
			TimedOut:       !completed || result.timedOut,
		})
		if state != contracts.CollectorStateUnavailable {
			capabilities = append(capabilities, "collector:"+result.name)
			points = append(points, result.points...)
			if result.apply != nil {
				result.apply(structured)
			}
		}
	}
	return points, statuses, capabilities, structured
}

func collectHostMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "host"}
	stats, err := host.InfoWithContext(ctx)
	if err != nil {
		result.failures++
		return result
	}
	result.points = appendNumericFields(result.points, "host_info_", stats, nil, func(name string) string {
		switch name {
		case "uptime":
			return "host_uptime_seconds"
		case "boot_time":
			return "host_boot_time_seconds"
		case "procs":
			return "host_processes"
		default:
			return "host_info_" + name
		}
	})
	metrics := &monitorv1.HostMetrics{
		UptimeSeconds:   stats.Uptime,
		BootTimeSeconds: stats.BootTime,
		ProcessCount:    stats.Procs,
	}
	result.apply = func(system *monitorv1.SystemMetrics) { system.Host = metrics }
	return result
}

func collectCPUMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "cpu"}
	metrics := &monitorv1.CpuMetrics{}
	cores := make(map[string]*monitorv1.CpuCoreMetrics)
	core := func(name string) *monitorv1.CpuCoreMetrics {
		value, ok := cores[name]
		if !ok {
			value = &monitorv1.CpuCoreMetrics{Cpu: name}
			cores[name] = value
		}
		return value
	}
	if count, err := cpu.CountsWithContext(ctx, true); err == nil {
		result.points = appendPoint(result.points, "host_cpu_logical_count", float64(count), nil)
		metrics.LogicalCount = uint32(count)
	} else {
		result.failures++
	}
	if count, err := cpu.CountsWithContext(ctx, false); err == nil {
		result.points = appendPoint(result.points, "host_cpu_physical_count", float64(count), nil)
		metrics.PhysicalCount = uint32(count)
	} else {
		result.failures++
	}
	if values, err := cpu.PercentWithContext(ctx, 0, true); err == nil {
		var total float64
		for index, value := range values {
			name := "cpu" + strconv.Itoa(index)
			usage := clampPercent(value)
			result.points = appendPoint(result.points, "host_cpu_usage_pct", usage, map[string]string{"cpu": name})
			core(name).UsagePct = usage
			total += usage
		}
		if len(values) > 0 {
			metrics.UsagePct = total / float64(len(values))
		}
	} else {
		result.failures++
	}
	if values, err := cpu.TimesWithContext(ctx, true); err == nil {
		for _, value := range values {
			labels := map[string]string{"cpu": value.CPU}
			cpuCore := core(value.CPU)
			cpuCore.UserSeconds = value.User
			cpuCore.SystemSeconds = value.System
			cpuCore.IdleSeconds = value.Idle
			cpuCore.NiceSeconds = value.Nice
			cpuCore.IowaitSeconds = value.Iowait
			cpuCore.IrqSeconds = value.Irq
			cpuCore.SoftirqSeconds = value.Softirq
			cpuCore.StealSeconds = value.Steal
			cpuCore.GuestSeconds = value.Guest
			cpuCore.GuestNiceSeconds = value.GuestNice
			result.points = appendNumericFields(result.points, "host_cpu_time_", value, labels, func(name string) string {
				return "host_cpu_time_" + name + "_seconds"
			})
		}
	} else {
		result.failures++
	}
	if values, err := cpu.InfoWithContext(ctx); err == nil {
		for _, value := range values {
			name := "cpu" + strconv.FormatInt(int64(value.CPU), 10)
			labels := map[string]string{"cpu": name}
			cpuCore := core(name)
			if value.Mhz >= 0 {
				cpuCore.FrequencyMhz = value.Mhz
				result.points = appendPoint(result.points, "host_cpu_frequency_mhz", value.Mhz, labels)
			}
			if value.CacheSize > 0 {
				cpuCore.CacheSizeKib = uint64(value.CacheSize)
				result.points = appendPoint(result.points, "host_cpu_cache_size_kib", float64(value.CacheSize), labels)
			}
		}
	} else {
		result.failures++
	}
	coreNames := make([]string, 0, len(cores))
	for name := range cores {
		coreNames = append(coreNames, name)
	}
	sort.Strings(coreNames)
	for _, name := range coreNames {
		metrics.Cores = append(metrics.Cores, cores[name])
	}
	result.apply = func(system *monitorv1.SystemMetrics) { system.Cpu = metrics }
	return result
}

func collectMemoryMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "memory"}
	stats, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		result.failures++
		return result
	}
	result.points = appendNumericFields(result.points, "host_mem_", stats, nil, func(name string) string {
		if name == "used_percent" {
			return "host_mem_used_pct"
		}
		return "host_mem_" + name + "_bytes"
	})
	metrics := &monitorv1.MemoryMetrics{
		TotalBytes: stats.Total, AvailableBytes: stats.Available, UsedBytes: stats.Used,
		UsedPct: stats.UsedPercent, FreeBytes: stats.Free, ActiveBytes: stats.Active,
		InactiveBytes: stats.Inactive, WiredBytes: stats.Wired, LaundryBytes: stats.Laundry,
		BuffersBytes: stats.Buffers, CachedBytes: stats.Cached, WritebackBytes: stats.WriteBack,
		DirtyBytes: stats.Dirty, SharedBytes: stats.Shared, SlabBytes: stats.Slab,
		PageTablesBytes: stats.PageTables, SwapCachedBytes: stats.SwapCached,
		CommitLimitBytes: stats.CommitLimit, CommittedAsBytes: stats.CommittedAS,
		MappedBytes: stats.Mapped, VmallocTotalBytes: stats.VmallocTotal,
		VmallocUsedBytes: stats.VmallocUsed, HugePagesTotal: stats.HugePagesTotal,
		HugePagesFree: stats.HugePagesFree, HugePagesReserved: stats.HugePagesRsvd,
		HugePagesSurplus: stats.HugePagesSurp, HugePageSizeBytes: stats.HugePageSize,
		AnonHugePagesBytes:      stats.AnonHugePages,
		WritebackTemporaryBytes: stats.WriteBackTmp,
		ReclaimableSlabBytes:    stats.Sreclaimable,
		UnreclaimableSlabBytes:  stats.Sunreclaim,
		HighTotalBytes:          stats.HighTotal,
		HighFreeBytes:           stats.HighFree,
		LowTotalBytes:           stats.LowTotal,
		LowFreeBytes:            stats.LowFree,
		SwapTotalBytes:          stats.SwapTotal,
		SwapFreeBytes:           stats.SwapFree,
		VmallocChunkBytes:       stats.VmallocChunk,
	}
	result.apply = func(system *monitorv1.SystemMetrics) { system.Memory = metrics }
	return result
}

func collectSwapMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "swap"}
	stats, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		result.failures++
		return result
	}
	result.points = appendNumericFields(result.points, "host_swap_", stats, nil, func(name string) string {
		switch name {
		case "used_percent":
			return "host_swap_used_pct"
		case "total", "used", "free", "sin", "sout":
			return "host_swap_" + name + "_bytes"
		default:
			return "host_swap_" + name + "_total"
		}
	})
	metrics := &monitorv1.SwapMetrics{
		TotalBytes: stats.Total, UsedBytes: stats.Used, FreeBytes: stats.Free,
		UsedPct: stats.UsedPercent, BytesInTotal: stats.Sin, BytesOutTotal: stats.Sout,
		PagesInTotal: stats.PgIn, PagesOutTotal: stats.PgOut,
		PageFaultsTotal: stats.PgFault, MajorPageFaultsTotal: stats.PgMajFault,
	}
	result.apply = func(system *monitorv1.SystemMetrics) { system.Swap = metrics }
	return result
}

func collectLoadMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "load"}
	metrics := &monitorv1.LoadMetrics{}
	if stats, err := load.AvgWithContext(ctx); err == nil {
		result.points = appendPoint(result.points, "host_load1", stats.Load1, nil)
		result.points = appendPoint(result.points, "host_load5", stats.Load5, nil)
		result.points = appendPoint(result.points, "host_load15", stats.Load15, nil)
		metrics.Load1 = stats.Load1
		metrics.Load5 = stats.Load5
		metrics.Load15 = stats.Load15
	} else {
		result.failures++
	}
	if stats, err := load.MiscWithContext(ctx); err == nil {
		result.points = appendNumericFields(result.points, "host_scheduler_", stats, nil, nil)
		metrics.ProcessesTotal = int64(stats.ProcsTotal)
		metrics.ProcessesCreatedTotal = int64(stats.ProcsCreated)
		metrics.ProcessesRunning = int64(stats.ProcsRunning)
		metrics.ProcessesBlocked = int64(stats.ProcsBlocked)
		metrics.ContextSwitchesTotal = int64(stats.Ctxt)
	} else {
		result.failures++
	}
	result.apply = func(system *monitorv1.SystemMetrics) { system.Load = metrics }
	return result
}

func collectFilesystemMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "filesystem"}
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		result.failures++
		return result
	}
	rootPath := systemRootPath()
	hasRoot := false
	for _, partition := range partitions {
		if filepath.Clean(partition.Mountpoint) == filepath.Clean(rootPath) {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		partitions = append(partitions, disk.PartitionStat{Mountpoint: rootPath})
	}
	seen := make(map[string]struct{}, len(partitions))
	var metrics []*monitorv1.FilesystemMetrics
	for _, partition := range partitions {
		if _, ok := seen[partition.Mountpoint]; ok {
			continue
		}
		seen[partition.Mountpoint] = struct{}{}
		stats, err := disk.UsageWithContext(ctx, partition.Mountpoint)
		if err != nil {
			result.failures++
			continue
		}
		labels := map[string]string{
			"device":     partition.Device,
			"mountpoint": partition.Mountpoint,
			"fstype":     partition.Fstype,
		}
		result.points = appendFilesystemUsage(result.points, stats, labels)
		metrics = append(metrics, &monitorv1.FilesystemMetrics{
			Device: partition.Device, Mountpoint: partition.Mountpoint, FilesystemType: partition.Fstype,
			TotalBytes: stats.Total, UsedBytes: stats.Used, FreeBytes: stats.Free,
			UsedPct: stats.UsedPercent, InodesTotal: stats.InodesTotal,
			InodesUsed: stats.InodesUsed, InodesFree: stats.InodesFree,
			InodesUsedPct: stats.InodesUsedPercent,
		})
	}
	result.apply = func(system *monitorv1.SystemMetrics) { system.Filesystems = metrics }
	return result
}

func collectDiskIOMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "disk_io"}
	stats, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		result.failures++
		return result
	}
	metrics := make([]*monitorv1.DiskIOMetrics, 0, len(stats))
	for name, value := range stats {
		labels := map[string]string{"device": name}
		result.points = appendNumericFields(result.points, "host_disk_io_", value, labels, func(field string) string {
			switch {
			case strings.HasSuffix(field, "_bytes"):
				return "host_disk_io_" + field + "_total"
			case strings.HasSuffix(field, "_time"), field == "io_time", field == "weighted_io":
				return "host_disk_io_" + field + "_milliseconds_total"
			default:
				return "host_disk_io_" + field + "_total"
			}
		})
		metrics = append(metrics, &monitorv1.DiskIOMetrics{
			Device: name, ReadsTotal: value.ReadCount, MergedReadsTotal: value.MergedReadCount,
			WritesTotal: value.WriteCount, MergedWritesTotal: value.MergedWriteCount,
			ReadBytesTotal: value.ReadBytes, WriteBytesTotal: value.WriteBytes,
			ReadMillisecondsTotal: value.ReadTime, WriteMillisecondsTotal: value.WriteTime,
			IoInProgress: value.IopsInProgress, IoMillisecondsTotal: value.IoTime,
			WeightedIoMillisecondsTotal: value.WeightedIO,
		})
	}
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].Device < metrics[j].Device })
	result.apply = func(system *monitorv1.SystemMetrics) { system.Disks = metrics }
	return result
}

func collectNetworkMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "network"}
	stats, err := gnet.IOCountersWithContext(ctx, true)
	if err != nil {
		result.failures++
		return result
	}
	interfacesByName := make(map[string]*monitorv1.NetworkInterfaceMetrics, len(stats))
	for _, value := range stats {
		labels := map[string]string{"interface": value.Name}
		result.points = appendNumericFields(result.points, "host_net_", value, labels, func(field string) string {
			return "host_net_" + field + "_total"
		})
		interfacesByName[value.Name] = &monitorv1.NetworkInterfaceMetrics{
			Interface: value.Name, BytesSentTotal: value.BytesSent, BytesReceivedTotal: value.BytesRecv,
			PacketsSentTotal: value.PacketsSent, PacketsReceivedTotal: value.PacketsRecv,
			ReceiveErrorsTotal: value.Errin, TransmitErrorsTotal: value.Errout,
			ReceiveDropsTotal: value.Dropin, TransmitDropsTotal: value.Dropout,
			ReceiveFifoErrorsTotal: value.Fifoin, TransmitFifoErrorsTotal: value.Fifoout,
		}
	}
	if interfaces, err := gnet.InterfacesWithContext(ctx); err == nil {
		for _, value := range interfaces {
			labels := map[string]string{"interface": value.Name}
			result.points = appendPoint(result.points, "host_net_interface_index", float64(value.Index), labels)
			result.points = appendPoint(result.points, "host_net_interface_mtu_bytes", float64(value.MTU), labels)
			metrics, ok := interfacesByName[value.Name]
			if !ok {
				metrics = &monitorv1.NetworkInterfaceMetrics{Interface: value.Name}
				interfacesByName[value.Name] = metrics
			}
			metrics.Index = int64(value.Index)
			metrics.MtuBytes = int64(value.MTU)
		}
	} else {
		result.failures++
	}
	var protocolMetrics []*monitorv1.NetworkProtocolMetrics
	if protocols, err := gnet.ProtoCountersWithContext(ctx, nil); err == nil {
		for _, protocol := range protocols {
			labels := map[string]string{"protocol": protocol.Protocol}
			for name, value := range protocol.Stats {
				result.points = appendPoint(result.points, "host_net_protocol_"+camelToSnake(name)+"_total", float64(value), labels)
			}
			protocolMetrics = append(protocolMetrics, &monitorv1.NetworkProtocolMetrics{
				Protocol: protocol.Protocol, Counters: cloneInt64Map(protocol.Stats),
			})
		}
	} else {
		result.failures++
	}
	conntrackMetrics := &monitorv1.NetworkConntrackMetrics{}
	if values, err := gnet.FilterCountersWithContext(ctx); err == nil {
		for _, value := range values {
			result.points = appendNumericFields(result.points, "host_net_filter_", value, nil, nil)
			conntrackMetrics.Count += value.ConnTrackCount
			conntrackMetrics.Maximum += value.ConnTrackMax
		}
	} else {
		result.failures++
	}
	if values, err := gnet.ConntrackStatsWithContext(ctx, false); err == nil {
		for _, value := range values {
			result.points = appendNumericFields(result.points, "host_net_conntrack_", value, nil, func(field string) string {
				return "host_net_conntrack_" + field + "_total"
			})
			conntrackMetrics.Entries += uint64(value.Entries)
			conntrackMetrics.SearchedTotal += uint64(value.Searched)
			conntrackMetrics.FoundTotal += uint64(value.Found)
			conntrackMetrics.NewTotal += uint64(value.New)
			conntrackMetrics.InvalidTotal += uint64(value.Invalid)
			conntrackMetrics.IgnoredTotal += uint64(value.Ignore)
			conntrackMetrics.DeletedTotal += uint64(value.Delete)
			conntrackMetrics.InsertedTotal += uint64(value.Insert)
			conntrackMetrics.InsertFailedTotal += uint64(value.InsertFailed)
			conntrackMetrics.DroppedTotal += uint64(value.Drop)
			conntrackMetrics.EarlyDroppedTotal += uint64(value.EarlyDrop)
			conntrackMetrics.SearchRestartsTotal += uint64(value.SearchRestart)
			conntrackMetrics.DeleteListTotal += uint64(value.DeleteList)
			conntrackMetrics.IcmpErrorsTotal += uint64(value.IcmpError)
			conntrackMetrics.ExpectedNewTotal += uint64(value.ExpectNew)
			conntrackMetrics.ExpectationsCreatedTotal += uint64(value.ExpectCreate)
			conntrackMetrics.ExpectationsDeletedTotal += uint64(value.ExpectDelete)
		}
	} else {
		result.failures++
	}
	interfaceNames := make([]string, 0, len(interfacesByName))
	for name := range interfacesByName {
		interfaceNames = append(interfaceNames, name)
	}
	sort.Strings(interfaceNames)
	interfaceMetrics := make([]*monitorv1.NetworkInterfaceMetrics, 0, len(interfaceNames))
	for _, name := range interfaceNames {
		interfaceMetrics = append(interfaceMetrics, interfacesByName[name])
	}
	sort.Slice(protocolMetrics, func(i, j int) bool { return protocolMetrics[i].Protocol < protocolMetrics[j].Protocol })
	result.apply = func(system *monitorv1.SystemMetrics) {
		system.NetworkInterfaces = interfaceMetrics
		system.NetworkProtocols = protocolMetrics
		system.NetworkConntrack = conntrackMetrics
	}
	return result
}

func collectSensorMetrics(ctx context.Context) collectorResult {
	result := collectorResult{name: "sensors"}
	stats, err := sensors.TemperaturesWithContext(ctx)
	if err != nil {
		result.failures++
		return result
	}
	metrics := make([]*monitorv1.TemperatureMetrics, 0, len(stats))
	for _, value := range stats {
		labels := map[string]string{"sensor": value.SensorKey}
		result.points = appendNumericFields(result.points, "host_sensor_", value, labels, func(field string) string {
			switch field {
			case "temperature":
				return "host_sensor_temperature_celsius"
			case "sensor_high":
				return "host_sensor_temperature_high_celsius"
			case "sensor_critical":
				return "host_sensor_temperature_critical_celsius"
			default:
				return "host_sensor_" + field
			}
		})
		metrics = append(metrics, &monitorv1.TemperatureMetrics{
			Sensor: value.SensorKey, CurrentCelsius: value.Temperature,
			HighCelsius: value.High, CriticalCelsius: value.Critical,
		})
	}
	result.apply = func(system *monitorv1.SystemMetrics) { system.Temperatures = metrics }
	return result
}

func appendFilesystemUsage(points []contracts.MetricPoint, stats *disk.UsageStat, labels map[string]string) []contracts.MetricPoint {
	points = appendPoint(points, "host_filesystem_total_bytes", float64(stats.Total), labels)
	points = appendPoint(points, "host_filesystem_used_bytes", float64(stats.Used), labels)
	points = appendPoint(points, "host_filesystem_free_bytes", float64(stats.Free), labels)
	points = appendPoint(points, "host_filesystem_used_pct", stats.UsedPercent, labels)
	points = appendPoint(points, "host_filesystem_inodes_total", float64(stats.InodesTotal), labels)
	points = appendPoint(points, "host_filesystem_inodes_used", float64(stats.InodesUsed), labels)
	points = appendPoint(points, "host_filesystem_inodes_free", float64(stats.InodesFree), labels)
	return appendPoint(points, "host_filesystem_inodes_used_pct", stats.InodesUsedPercent, labels)
}

func appendNumericFields(points []contracts.MetricPoint, prefix string, input any, labels map[string]string, nameFunc func(string) string) []contracts.MetricPoint {
	value := reflect.ValueOf(input)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return points
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return points
	}
	typeOf := value.Type()
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		metricValue, ok := numericValue(field)
		if !ok {
			continue
		}
		name := jsonFieldName(typeOf.Field(index))
		if nameFunc != nil {
			name = nameFunc(name)
		} else {
			name = prefix + name
		}
		points = appendPoint(points, name, metricValue, labels)
	}
	return points
}

func numericValue(value reflect.Value) (float64, bool) {
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(value.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(value.Uint()), true
	case reflect.Float32, reflect.Float64:
		return value.Float(), true
	default:
		return 0, false
	}
}

func jsonFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("json"), ",")[0]
	if name == "" || name == "-" {
		name = field.Name
	}
	return camelToSnake(name)
}

func camelToSnake(name string) string {
	var output strings.Builder
	for index, char := range name {
		if unicode.IsUpper(char) && index > 0 {
			output.WriteByte('_')
		}
		output.WriteRune(unicode.ToLower(char))
	}
	return output.String()
}

func appendPoint(points []contracts.MetricPoint, name string, value float64, labels map[string]string) []contracts.MetricPoint {
	if name == "" || math.IsNaN(value) || math.IsInf(value, 0) {
		return points
	}
	return append(points, contracts.MetricPoint{Name: name, Value: value, Labels: cloneStringMap(labels)})
}

func uniqueMetricNames(points []contracts.MetricPoint) []string {
	seen := make(map[string]struct{}, len(points))
	for _, point := range points {
		seen[point.Name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cloneInt64Map(input map[string]int64) map[string]int64 {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]int64, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
