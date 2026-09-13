"""Exercise the Unix installer against local release fixtures, without network or sudo."""
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest


INSTALLER = Path(__file__).resolve().parents[1] / "install.sh"


class InstallerTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.mock = self.root / "mock"
        self.assets = self.root / "assets"
        self.dest = self.root / "bin"
        for path in (self.mock, self.assets, self.dest):
            path.mkdir()
        self.write_command("uname", '#!/bin/sh\ncase "$1" in -s) echo "$TEST_OS";; -m) echo "$TEST_ARCH";; esac\n')
        self.write_command("curl", '''#!/bin/sh
while [ "$#" -gt 0 ]; do
  case "$1" in
    https://*) url=$1 ;;
    -o) shift; output=$1 ;;
  esac
  shift
done
cp "$TEST_ASSETS/${url##*/}" "$output"
''')
        self.env = dict(os.environ, PATH=str(self.mock) + os.pathsep + os.environ["PATH"],
                        OPTIMUS_INSTALL_DIR=str(self.dest), TEST_ASSETS=str(self.assets),
                        TEST_OS="Linux", TEST_ARCH="x86_64")
        self.env.pop("OPTIMUS_VERSION", None)

    def write_command(self, name, content):
        path = self.mock / name
        path.write_text(content)
        path.chmod(0o755)

    def make_archive(self, os_name="linux", arch="amd64"):
        asset = f"optimus_{os_name}_{arch}.tar.gz"
        path = self.assets / asset
        content = b"#!/bin/sh\necho 'Optimus v0.1.0'\n"
        with tarfile.open(path, "w:gz") as archive:
            entry = tarfile.TarInfo("optimus")
            entry.size, entry.mode = len(content), 0o755
            archive.addfile(entry, io.BytesIO(content))
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        (self.assets / "checksums.txt").write_text(f"{digest}  {asset}\n")
        return path

    def run_installer(self):
        return subprocess.run(["sh"], input=INSTALLER.read_text(), env=self.env,
                              text=True, capture_output=True, timeout=10)

    def test_install_and_update(self):
        self.make_archive()
        for _ in range(2):
            result = self.run_installer()
            self.assertEqual(result.returncode, 0, result.stderr)
            binary = self.dest / "optimus"
            self.assertTrue(os.access(binary, os.X_OK))
            self.assertEqual(subprocess.check_output([str(binary)], text=True).strip(), "Optimus v0.1.0")

    def test_macos_arm64_and_pinned_version(self):
        self.make_archive("darwin", "arm64")
        self.env.update(TEST_OS="Darwin", TEST_ARCH="arm64", OPTIMUS_VERSION="v0.1.0")
        result = self.run_installer()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.dest / "optimus").exists())

    def test_checksum_failure_preserves_existing_binary(self):
        path = self.make_archive()
        path.write_bytes(path.read_bytes() + b"corrupted")
        binary = self.dest / "optimus"
        binary.write_text("existing binary")
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Checksum mismatch", result.stderr)
        self.assertEqual(binary.read_text(), "existing binary")

    def test_unsupported_architecture(self):
        self.env["TEST_ARCH"] = "riscv64"
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.dest / "optimus").exists())

    def test_invalid_version(self):
        self.env["OPTIMUS_VERSION"] = "v0.1.0/other"
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.dest / "optimus").exists())


if __name__ == "__main__":
    unittest.main()
