"""Exercise the real release script with disposable paths and command fixtures."""

import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="office-release-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.opt = self.root / "opt"
        self.etc = self.root / "etc"
        self.bin = self.root / "bin"
        self.previous = self.opt / "releases" / "previous"
        for path in [self.previous / "deploy", self.etc, self.bin]:
            path.mkdir(parents=True, exist_ok=True)
        self.old_unit = "[Service]\n# previous unit\n"
        (self.previous / "deploy/office-share.service").write_text(self.old_unit)
        (self.opt / "current").symlink_to(self.previous)
        (self.etc / "office-share.env").write_text("")
        (self.etc / "office-share.service").write_text(self.old_unit)
        stage = self.root / "stage"
        (stage / "deploy").mkdir(parents=True)
        (stage / "office-share").write_text("#!/bin/sh\nexit 0\n")
        (stage / "deploy/run.sh").write_text("#!/bin/sh\nexit 0\n")
        (stage / "deploy/office-share.service").write_text("[Service]\n# new unit\n")
        self.archive = self.root / "release.tar.gz"
        with tarfile.open(self.archive, "w:gz") as archive:
            for path in stage.iterdir():
                archive.add(path, arcname=path.name)
        script = Path(__file__).with_name("release.sh").read_text()
        self.assertIn('[[ "$EUID" -eq 0 ]]', script)
        # Do not touch the host's /opt, /etc, services, permissions, or network.
        script = script.replace('[[ "$EUID" -eq 0 ]]', "true")
        script = script.replace("/opt/office-share", str(self.opt))
        script = script.replace("/etc/office-share.env", str(self.etc / "office-share.env"))
        script = script.replace(
            "/etc/systemd/system/office-share.service", str(self.etc / "office-share.service")
        )
        self.script = self.root / "release.sh"
        self.script.write_text(script)
        self.command("chown", "exit 0")
        self.command("sleep", "exit 0")
        self.command(
            "systemctl",
            '''echo "systemctl $*" >> "$FIXTURE/commands.log"
count_file="$FIXTURE/$1.count"
count=0
if [ -f "$count_file" ]; then read -r count < "$count_file"; fi
count=$((count + 1))
echo "$count" > "$count_file"
if [ "$SCENARIO" = "$1" ] && [ "$count" = 1 ]; then exit 1; fi
exit 0''',
        )
        self.command(
            "curl",
            '''echo curl >> "$FIXTURE/commands.log"
if [ "$SCENARIO" = health ]; then exit 1; fi
exit 0''',
        )

    def command(self, name, body):
        path = self.bin / name
        path.write_text("#!/bin/sh\n" + body + "\n")
        path.chmod(0o755)

    def release(self, scenario):
        return subprocess.run(
            ["bash", str(self.script), str(self.archive)],
            env={
                **os.environ,
                "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
                "FIXTURE": str(self.root),
                "SCENARIO": scenario,
            },
            capture_output=True,
            text=True,
            timeout=10,
        )

    def assert_rolled_back(self, result):
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.opt / "current").resolve(), self.previous)
        self.assertEqual((self.etc / "office-share.service").read_text(), self.old_unit)
        self.assertIn("systemctl restart office-share", (self.root / "commands.log").read_text())

    def test_restart_failure_restores_previous_release_and_unit(self):
        self.assert_rolled_back(self.release("restart"))
        self.assertEqual((self.root / "restart.count").read_text().strip(), "2")

    def test_daemon_reload_failure_rolls_back(self):
        self.assert_rolled_back(self.release("daemon-reload"))

    def test_health_failure_restores_previous_release_and_unit(self):
        self.assert_rolled_back(self.release("health"))

    def test_success_keeps_new_release_and_unit(self):
        result = self.release("success")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotEqual((self.opt / "current").resolve(), self.previous)
        self.assertIn("# new unit", (self.etc / "office-share.service").read_text())
        self.assertEqual((self.root / "restart.count").read_text().strip(), "1")


if __name__ == "__main__":
    unittest.main()
