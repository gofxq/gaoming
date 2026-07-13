import { Button, Tooltip } from "@douyinfe/semi-ui";
import { IconLayers, IconMoonStroked, IconSunStroked, IconTerminal } from "@douyinfe/semi-icons";
import { useAppearance } from "../../../shared/features/appearance/AppearanceProvider";

type AppearanceControlsProps = {
  className?: string;
};

export function AppearanceControls({ className = "" }: AppearanceControlsProps) {
  const { skin, theme, toggleSkin, toggleTheme } = useAppearance();
  const skinLabel = skin === "pixel" ? "切换现代皮肤" : "切换像素皮肤";
  const themeLabel = theme === "dark" ? "切换浅色主题" : "切换深色主题";

  return (
    <div className={`appearance-controls ${className}`.trim()} role="group" aria-label="界面外观">
      <Tooltip content={skinLabel}>
        <Button
          className="icon-button quiet skin-toggle"
          icon={skin === "pixel" ? <IconLayers /> : <IconTerminal />}
          aria-label={skinLabel}
          aria-pressed={skin === "pixel"}
          onClick={toggleSkin}
        />
      </Tooltip>
      <Tooltip content={themeLabel}>
        <Button
          className="icon-button quiet theme-toggle"
          icon={theme === "dark" ? <IconSunStroked /> : <IconMoonStroked />}
          aria-label={themeLabel}
          aria-pressed={theme === "dark"}
          onClick={toggleTheme}
        />
      </Tooltip>
    </div>
  );
}
