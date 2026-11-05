import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import textwrap
import unittest


WORKFLOW = Path(__file__).resolve().parents[1] / 'workflows' / 'build.yaml'


class ReleaseVersionTest(unittest.TestCase):
    def test_all_build_jobs_preserve_tag_as_data(self):
        text = WORKFLOW.read_text()
        jobs = re.split(r'(?m)^  (?=build-(?:linux|darwin)[^:]*:\s*$)', text)[1:]
        self.assertEqual(len(jobs), 4)
        for job in jobs:
            for tag in ['v1.2.3', "v1'$(printf${IFS}RELEASE_EXECUTED>&2)'"]:
                with self.subTest(job=job.splitlines()[0], tag=tag):
                    step = job.split('    - name: advance version\n', 1)[1].split('\n    - ', 1)[0]
                    script = textwrap.dedent(step.split('      run: |\n', 1)[1])
                    script = script.replace('${{github.event.release.tag_name}}', tag)
                    with tempfile.TemporaryDirectory() as directory:
                        root = Path(directory)
                        config = root / 'pkg' / 'config' / 'config.json'
                        config.parent.mkdir(parents=True)
                        config.write_text('{"version":"old", "preserved":true}\n')
                        env = {key: value for key, value in os.environ.items() if key != 'RELEASE_TAG'}
                        if re.search(r'(?m)^      RELEASE_TAG: \$\{\{ github.event.release.tag_name \}\}$', job):
                            env['RELEASE_TAG'] = tag
                        result = subprocess.run(['bash', '-e', '-o', 'pipefail', '-c', script],
                                                cwd=root, env=env, capture_output=True, text=True)
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertNotIn('RELEASE_EXECUTED', result.stderr)
                        self.assertEqual(json.loads(config.read_text()),
                                         {'version': tag, 'preserved': True})


if __name__ == '__main__':
    unittest.main()
