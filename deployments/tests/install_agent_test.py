"""Exercise installer functions without network access or host service changes."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "install-agent.sh"


class InstallerTest(unittest.TestCase):
    def run_shell(self, body, **env):
        source = SCRIPT.read_text().rsplit('main "$@"', 1)[0]
        with tempfile.TemporaryDirectory(prefix="gaoming-install-test-") as tmp:
            process = subprocess.run(
                ["sh"], input=source + "\n" + body, text=True,
                capture_output=True,
                env={**os.environ, "TEST_DIR": tmp, **env},
            )
        return process

    def test_detects_supported_platforms(self):
        for kernel, machine, asset in [
            ("Linux", "x86_64", "gaoming-agent_linux_amd64.tar.gz"),
            ("Linux", "aarch64", "gaoming-agent_linux_arm64.tar.gz"),
            ("Darwin", "x86_64", "gaoming-agent_darwin_amd64.tar.gz"),
            ("Darwin", "arm64", "gaoming-agent_darwin_arm64.tar.gz"),
        ]:
            with self.subTest(kernel=kernel, machine=machine):
                result = self.run_shell('''
uname() { case "$1" in -s) echo "$KERNEL" ;; -m) echo "$MACHINE" ;; esac; }
prepare_platform
echo "$ASSET"
''', KERNEL=kernel, MACHINE=machine)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.strip(), asset)

    def test_default_install_never_prompts(self):
        result = self.run_shell('''
has_tty_prompt() { die "unexpected terminal prompt"; }
INSTALL_DIR="$TEST_DIR"
choose_install_mode
prompt_install_inputs
[ "$INSTALL_MODE" = reinstall ]
echo 'tenant_code: "team"' > "$INSTALL_DIR/agent-config.yaml"
INSTALL_MODE=auto
choose_install_mode
[ "$INSTALL_MODE" = update-binary ]
''')
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_explicit_update_preserves_config_without_prompt(self):
        result = self.run_shell('''
has_tty_prompt() { die "unexpected terminal prompt"; }
INSTALL_DIR="$TEST_DIR"
echo 'tenant_code: "team"' > "$INSTALL_DIR/agent-config.yaml"
parse_args --update-binary
choose_install_mode
[ "$INSTALL_MODE" = update-binary ]
''')
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_binary_update_preserves_config_and_replaces_file_atomically(self):
        result = self.run_shell('''
INSTALL_DIR="$TEST_DIR/installed"
TMP_DIR="$TEST_DIR/download"
mkdir -p "$INSTALL_DIR" "$TMP_DIR/package"
echo 'tenant_code: "team"' > "$INSTALL_DIR/agent-config.yaml"
cp "$INSTALL_DIR/agent-config.yaml" "$TEST_DIR/original-config"
echo old > "$INSTALL_DIR/gaoming-agent"
ln "$INSTALL_DIR/gaoming-agent" "$TEST_DIR/old-inode"
echo new > "$TMP_DIR/package/gaoming-agent"
ASSET=fixture.tar.gz
tar -czf "$TMP_DIR/$ASSET" -C "$TMP_DIR/package" gaoming-agent
INSTALL_MODE=update-binary
extract_binary
install_files
cmp "$INSTALL_DIR/agent-config.yaml" "$TEST_DIR/original-config"
[ "$(cat "$INSTALL_DIR/gaoming-agent")" = new ]
[ "$(cat "$TEST_DIR/old-inode")" = old ]
''')
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_bad_checksum_fails_before_install(self):
        result = self.run_shell('''
TMP_DIR="$TEST_DIR"
ASSET=fixture.tar.gz
echo payload > "$TMP_DIR/$ASSET"
printf '%064d  fixture.tar.gz\n' 0 > "$TMP_DIR/checksums.txt"
verify_checksum
echo 'must not reach install'
''')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("must not reach install", result.stdout)
        self.assertIn("checksum verification failed", result.stderr)

    def test_update_service_restarts_existing_process(self):
        result = self.run_shell('''
INSTALL_DIR="$TEST_DIR/installed"
mkdir -p "$INSTALL_DIR"
echo binary > "$INSTALL_DIR/gaoming-agent"
echo config > "$INSTALL_DIR/agent-config.yaml"
ensure_linux_identity() { :; }
chown() { :; }
write_file() { cat > "$TEST_DIR/unit"; }
systemctl() { echo "$*" >> "$TEST_DIR/systemctl"; }
install_linux_service
cat "$TEST_DIR/systemctl"
''')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("restart gaoming-agent", result.stdout)

    def test_unsafe_install_paths_rejected(self):
        for path in ["/", "/opt", "/usr", "/usr/local", "//opt", "/usr//local", "//etc", "/tmp/../", "/tmp/unit\nExecStart=bad"]:
            with self.subTest(path=path):
                result = self.run_shell('validate_abs_path "$UNSAFE" install-dir', UNSAFE=path)
                self.assertNotEqual(result.returncode, 0)

    def test_yaml_strings_escape_controls(self):
        result = self.run_shell('yaml_escape "$VALUE"', VALUE='a"b\\c\nextra: injected\tend\r')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, 'a\\"b\\\\c\\nextra: injected\\tend\\r')

    def test_symlink_install_directory_rejected(self):
        result = self.run_shell('''
mkdir "$TEST_DIR/target"
ln -s "$TEST_DIR/target" "$TEST_DIR/link"
INSTALL_DIR="$TEST_DIR/link"
validate_common_inputs
''')
        self.assertNotEqual(result.returncode, 0)

    def test_launchd_start_failure_is_reported(self):
        result = self.run_shell('''
INSTALL_DIR="$TEST_DIR"
echo config > "$INSTALL_DIR/agent-config.yaml"
chown() { :; }
write_file() { cat > "$TEST_DIR/plist"; }
launchctl() { case "$1" in kickstart) return 1 ;; *) return 0 ;; esac; }
install_darwin_service
''')
        self.assertNotEqual(result.returncode, 0)

    def test_missing_archive_binary_rejected_during_preparation(self):
        result = self.run_shell('''
TMP_DIR="$TEST_DIR"
mkdir "$TMP_DIR/package"
echo bad > "$TMP_DIR/package/other"
ASSET=fixture.tar.gz
tar -czf "$TMP_DIR/$ASSET" -C "$TMP_DIR/package" other
extract_binary
echo 'must not stop service'
''')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("must not stop service", result.stdout)

    def test_failed_upgrade_restores_binary_and_config(self):
        result = self.run_shell('''
INSTALL_DIR="$TEST_DIR/installed"
TMP_DIR="$TEST_DIR/download"
mkdir -p "$INSTALL_DIR" "$TMP_DIR/previous"
echo old > "$TMP_DIR/previous/gaoming-agent"
echo old-config > "$TMP_DIR/previous/agent-config.yaml"
echo new > "$INSTALL_DIR/gaoming-agent"
echo new-config > "$INSTALL_DIR/agent-config.yaml"
OS=linux
UPGRADE_IN_PROGRESS=1
systemctl() { echo "$*"; }
cleanup 1
[ "$(cat "$INSTALL_DIR/gaoming-agent")" = old ]
[ "$(cat "$INSTALL_DIR/agent-config.yaml")" = old-config ]
''')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("restart gaoming-agent", result.stdout)

    def test_one_command_install_and_update_with_release_fixture(self):
        # Substitute only OS/network boundaries; run the actual main flow,
        # checksum verification, archive preparation and file replacement.
        result = self.run_shell('''
TMPDIR="$TEST_DIR"
mkdir "$TEST_DIR/package"
echo first-binary > "$TEST_DIR/package/gaoming-agent"
tar -czf "$TEST_DIR/release.tar.gz" -C "$TEST_DIR/package" gaoming-agent
if command -v sha256sum >/dev/null; then
  digest="$(sha256sum "$TEST_DIR/release.tar.gz" | awk '{print $1}')"
else
  digest="$(shasum -a 256 "$TEST_DIR/release.tar.gz" | awk '{print $1}')"
fi
printf '%s  gaoming-agent_darwin_arm64.tar.gz\n' "$digest" > "$TEST_DIR/checksums.txt"
uname() { case "$1" in -s) echo Darwin ;; -m) echo arm64 ;; esac; }
id() { echo 0; }
chown() { :; }
launchctl() { return 0; }
curl() {
  request=""; destination=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      https://*) request="$1"; shift ;;
      -o) destination="$2"; shift 2 ;;
      *) shift ;;
    esac
  done
  case "$request" in
    */checksums.txt) cp "$TEST_DIR/checksums.txt" "$destination" ;;
    */gaoming-agent_darwin_arm64.tar.gz) cp "$TEST_DIR/release.tar.gz" "$destination" ;;
    *) die "unexpected download: $request" ;;
  esac
}
write_file() {
  case "$1" in
    /Library/*) cat > "$TEST_DIR/service.plist" ;;
    *) cat > "$1"; chmod "$2" "$1" ;;
  esac
}
main --install-dir "$TEST_DIR/installed" --tenant team --web-url https://monitor.example.com
[ "$(cat "$INSTALL_DIR/gaoming-agent")" = first-binary ]
grep -q 'tenant_code: "team"' "$INSTALL_DIR/agent-config.yaml"
grep -q '<key>RunAtLoad</key>' "$TEST_DIR/service.plist"
cp "$INSTALL_DIR/agent-config.yaml" "$TEST_DIR/preserved"
INSTALL_MODE=auto
main --install-dir "$TEST_DIR/installed" --tenant other --web-url https://other.example.com
cmp "$INSTALL_DIR/agent-config.yaml" "$TEST_DIR/preserved"
''')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("install mode: reinstall", result.stdout)
        self.assertIn("install mode: update-binary", result.stdout)


if __name__ == "__main__":
    unittest.main()
