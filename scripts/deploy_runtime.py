#!/usr/bin/env python3
"""Update existing Linux services. No SQL writes, credentials or forced kills."""
import json
import os
import pathlib
import shutil
import signal
import subprocess
import sys
import time
import urllib.request


class Deployment:
    def __init__(self, root, build, timeout=20):
        self.root = pathlib.Path(root).resolve()
        self.build = pathlib.Path(build).resolve()
        self.timeout = timeout
        self.new = []
        self.old = {}
        self.identities = {}
        self.backup = None
        self.services = [
            ('nehuang-server', 'server.log', os.getenv('NEHUANG_DEPLOY_HTTP_BASE_URL', 'http://127.0.0.1:8080')),
            ('gateway-adapter', 'gateway-adapter.log', os.getenv('NEHUANG_DEPLOY_ADAPTER_BASE_URL', 'http://127.0.0.1:18081')),
        ]

    def process_identity(self, pid):
        entry = pathlib.Path('/proc', str(pid))
        # starttime detects PID reuse; never signal a different executable/process.
        start = (entry / 'stat').read_text().split(') ', 1)[1].split()[19]
        return (os.readlink(entry / 'exe'), os.readlink(entry / 'cwd'), start)

    def discover(self, name):
        found = []
        for entry in pathlib.Path('/proc').iterdir():
            if not entry.name.isdigit():
                continue
            try:
                pid = int(entry.name)
                identity = self.process_identity(pid)
                if identity[:2] != (str(self.root / name), str(self.root)):
                    continue
                args = (entry / 'cmdline').read_bytes().rstrip(b'\0').split(b'\0')
                if len(args) != 1:
                    raise RuntimeError('existing service has unsupported arguments: ' + name)
                env = os.environ.copy()
                for item in (entry / 'environ').read_bytes().split(b'\0'):
                    key, sep, value = item.partition(b'=')
                    if sep and key.startswith(b'NEHUANG_'):
                        env[key.decode()] = value.decode()
                self.identities[pid] = identity
                found.append((pid, env))
            except (FileNotFoundError, ProcessLookupError):
                continue
        if len(found) != 1:
            raise RuntimeError('expected one existing service: ' + name)
        return found[0]

    def alive(self, pid):
        try:
            os.kill(pid, 0)
            return pathlib.Path('/proc', str(pid), 'stat').read_text().split(') ', 1)[1][0] != 'Z'
        except (FileNotFoundError, ProcessLookupError):
            return False

    def assert_identity(self, pid):
        if pid not in self.identities or self.process_identity(pid) != self.identities[pid]:
            raise RuntimeError('process identity changed; no signal sent')

    def stop(self, pid):
        if not self.alive(pid):
            return
        self.assert_identity(pid)
        os.kill(pid, signal.SIGTERM)
        deadline = time.monotonic() + self.timeout
        while self.alive(pid) and time.monotonic() < deadline:
            self.assert_identity(pid)
            time.sleep(.1)
        if self.alive(pid):
            raise RuntimeError('service did not stop gracefully; no forced kill')

    def probe(self, base):
        with urllib.request.urlopen(base + '/healthz', timeout=2) as response:
            return json.load(response)

    def preflight_shutdown(self):
        info = self.probe(self.services[1][2])
        if info.get('connection_shutdown_supported') is not True:
            # First upgrade: refuse before signals when legacy peers are connected.
            # A zero-count race is still handled by attempted-stop recovery below.
            if info.get('active_connections') != 0:
                raise RuntimeError('legacy adapter has active/unknown TCP peers; maintenance drain required before any stop')

    def start(self, name, log, env):
        with open(self.root / log, 'ab', buffering=0) as output:
            process = subprocess.Popen([str(self.root / name)], cwd=self.root, env=env,
                                       stdin=subprocess.DEVNULL, stdout=output, stderr=output, start_new_session=True)
        self.new.append(process.pid)
        self.identities[process.pid] = self.process_identity(process.pid)
        return process.pid

    def ready(self, pid, base):
        deadline = time.monotonic() + self.timeout
        while time.monotonic() < deadline:
            if not self.alive(pid):
                raise RuntimeError('service exited; logs retained')
            self.assert_identity(pid)
            try:
                for path in ('/healthz', '/readyz'):
                    with urllib.request.urlopen(base + path, timeout=1) as response:
                        if response.status != 200:
                            raise RuntimeError('not ready')
                return
            except Exception:
                time.sleep(.2)
        raise RuntimeError('service readiness timed out')

    def run(self):
        for name, _, _ in self.services:
            if not (self.build / name).is_file():
                raise RuntimeError('build output missing: ' + name)
            self.old[name] = self.discover(name)
        self.preflight_shutdown()
        self.backup = self.root / '.deploy-backups' / str(int(time.time() * 1000000))
        self.backup.mkdir(parents=True, mode=0o700)
        for name, _, _ in self.services:
            shutil.copy2(self.root / name, self.backup / name)
        attempted = set()
        try:
            for name, _, _ in reversed(self.services):
                # Record before SIGTERM; timeout does not mean untouched/healthy.
                attempted.add(name)
                self.stop(self.old[name][0])
            for name, log, base in self.services:
                shutil.copy2(self.build / name, self.root / (name + '.next'))
                os.replace(self.root / (name + '.next'), self.root / name)
                pid = self.start(name, log, self.old[name][1])
                self.ready(pid, base)
                (self.root / (name + '.pid')).write_text(str(pid) + '\n')
        except BaseException:
            rollback_ok = True
            for pid in reversed(self.new):
                try:
                    self.stop(pid)
                except Exception:
                    rollback_ok = False
            for name in attempted:
                try:
                    # A signalled legacy process must actually exit before restart.
                    self.stop(self.old[name][0])
                except Exception:
                    rollback_ok = False
            restored = {name: self.old[name][0] for name, _, _ in self.services}
            if rollback_ok:
                for name, log, _ in self.services:
                    if name not in attempted:
                        continue
                    try:
                        shutil.copy2(self.backup / name, self.root / (name + '.next'))
                        os.replace(self.root / (name + '.next'), self.root / name)
                        pid = self.start(name, log, self.old[name][1])
                        restored[name] = pid
                        (self.root / (name + '.pid')).write_text(str(pid) + '\n')
                    except Exception:
                        rollback_ok = False
                # Check BOTH originals, including a service never restarted.
                for name, _, base in self.services:
                    try:
                        self.ready(restored[name], base)
                    except Exception:
                        rollback_ok = False
            print('Deployment failed; rollback ' + ('healthy' if rollback_ok else 'requires operator attention') +
                  '; backup: ' + str(self.backup), file=sys.stderr)
            raise
        print(json.dumps({'deployed': True, 'backup': str(self.backup), 'services': [s[0] for s in self.services]}))


if __name__ == '__main__':
    def interrupted(*_):
        raise RuntimeError('deployment interrupted')
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    try:
        Deployment(sys.argv[1], sys.argv[2]).run()
    except BaseException as error:
        print('Deployment stopped: ' + str(error), file=sys.stderr)
        sys.exit(1)
