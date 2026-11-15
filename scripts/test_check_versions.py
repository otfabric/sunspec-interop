#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Tests of scripts/check-versions.sh: it runs on a copy of the repository, as
it is and with one pin changed in one place."""

import re
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def own_dockerfile(root):
    """The Dockerfile of the one own stack (own/<stack>/Dockerfile)."""
    found = sorted(root.glob("own/*/Dockerfile"))
    if len(found) != 1:
        raise AssertionError("want one own/*/Dockerfile, found %s" % found)
    return found[0]


class CheckVersions(unittest.TestCase):

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name) / "repo"
        shutil.copytree(ROOT, self.root, ignore=shutil.ignore_patterns(".git", "reports", "__pycache__"))

    def tearDown(self):
        self.tmp.cleanup()

    def run_check(self):
        p = subprocess.run(["bash", str(self.root / "scripts" / "check-versions.sh")], capture_output=True, text=True)
        return p.returncode, p.stdout + p.stderr

    def edit(self, path, old, new):
        path = self.root / path
        text = path.read_text()
        self.assertIn(old, text, path)
        path.write_text(text.replace(old, new, 1))

    def assert_fails_with(self, needle):
        code, out = self.run_check()
        self.assertEqual(code, 1, out)
        self.assertIn(needle, out)

    def test_the_repository_passes(self):
        code, out = self.run_check()
        self.assertEqual(code, 0, out)

    def test_a_missing_revision_fails(self):
        self.edit("versions.yaml", '    revision: "bbe9e044c2ae06af54b49e5ef042493eddb7e2b8"', '    upstream_tag: "v1.3.7"')
        self.assert_fails_with("FAIL pysunspec2: revision '' is not a commit")

    def test_a_revision_that_is_not_a_commit_fails(self):
        self.edit("versions.yaml", '    revision: "c99cbef0ba8fefb8433663db152f9fdde8c7adb1"', '    revision: "v0.3.0"')
        self.assert_fails_with("FAIL cbsigma: revision 'v0.3.0' is not a commit")

    def test_a_missing_lock_fails(self):
        self.edit("versions.yaml", '    lock: "adapters/cbsigma/go.sum"', '    lock: "adapters/cbsigma/go.sum.old"')
        self.assert_fails_with("FAIL cbsigma: lock 'adapters/cbsigma/go.sum.old' is not a file of this repository")

    def test_a_lock_without_the_version_fails(self):
        self.edit("adapters/pysunspec2/requirements.txt", "pysunspec2==1.3.7", "pysunspec2==1.3.8")
        self.assert_fails_with("FAIL pysunspec2: adapters/pysunspec2/requirements.txt does not pin version 1.3.7")

    def test_a_wheel_hash_versions_yaml_does_not_record_fails(self):
        self.edit("adapters/pysunspec2/requirements.txt", "sha256:4bc5029ac2ee", "sha256:0bc5029ac2ee")
        self.assert_fails_with("FAIL pysunspec2: adapters/pysunspec2/requirements.txt has pysunspec2 1.3.7 sha256:0bc5029ac2ee")

    def test_a_bundled_wheel_hash_versions_yaml_does_not_record_fails(self):
        self.edit("adapters/pysunspec2/requirements.txt", "sha256:c4451db6ba39", "sha256:04451db6ba39")
        self.assert_fails_with("which is not a bundled package of versions.yaml with that version and hash")

    def test_a_bundled_version_versions_yaml_does_not_record_fails(self):
        self.edit("versions.yaml", '      pyserial: "3.5"', '      pyserial: "3.4"')
        self.assert_fails_with("FAIL pysunspec2: adapters/pysunspec2/requirements.txt has pyserial 3.5 sha256:")

    def test_an_own_stack_digest_versions_yaml_does_not_list_fails(self):
        dockerfile = own_dockerfile(self.root)
        text = dockerfile.read_text()
        digest = re.search(r"^ARG GOLANG_DIGEST=(sha256:[0-9a-f]{64})$", text, re.M).group(1)
        dockerfile.write_text(text.replace(digest, "sha256:" + "1" * 64))
        self.assert_fails_with("base image sha256:%s is not in base_images with otfabric in used_by" % ("1" * 64))

    def test_an_own_stack_base_image_without_the_stack_in_used_by_fails(self):
        self.edit("versions.yaml", "    used_by: [cbsigma, otfabric]\n  debian:", "    used_by: [cbsigma]\n  debian:")
        self.assert_fails_with("is not in base_images with otfabric in used_by")

    def test_a_base_image_used_by_an_own_stack_that_does_not_use_it_fails(self):
        self.edit("versions.yaml", "    used_by: [pysunspec2]", "    used_by: [pysunspec2, otfabric]")
        self.assert_fails_with("is used_by otfabric but is not a base image of own/")

    def test_an_own_stack_without_an_entry_fails(self):
        dockerfile = own_dockerfile(self.root)
        build = "/".join(dockerfile.relative_to(self.root).parts[:2])
        self.edit("versions.yaml", '    build: "%s"' % build, '    build: "own/elsewhere"')
        self.assert_fails_with("no own_implementations entry has build: %s" % build)


if __name__ == "__main__":
    unittest.main()
