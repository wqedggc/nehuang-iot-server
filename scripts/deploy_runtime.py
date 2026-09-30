#!/usr/bin/env python3
"""Update existing Linux services only. No SQL writes or account provisioning."""
import json, os, pathlib, shutil, signal, subprocess, sys, time, urllib.request

class Deployment:
    def __init__(self, root, build, timeout=20):
        self.root = pathlib.Path(root).resolve(); self.build = pathlib.Path(build).resolve()
        self.timeout = timeout; self.new = []; self.old = {}; self.backup = None
        self.services = [('nehuang-server', 'server.log', os.getenv('NEHUANG_DEPLOY_HTTP_BASE_URL','http://127.0.0.1:8080')), ('gateway-adapter','gateway-adapter.log',os.getenv('NEHUANG_DEPLOY_ADAPTER_BASE_URL','http://127.0.0.1:18081'))]
    def discover(self, name):
        target = self.root / name
        found = []
        for entry in pathlib.Path('/proc').iterdir():
            if not entry.name.isdigit(): continue
            try:
                exe = os.readlink(entry / 'exe')
                if exe != str(target) or os.readlink(entry/'cwd') != str(self.root): continue
                args = (entry/'cmdline').read_bytes().rstrip(b'\0').split(b'\0')
                if len(args) != 1: raise RuntimeError('existing service has unsupported arguments: '+name)
                env = os.environ.copy()
                for item in (entry/'environ').read_bytes().split(b'\0'):
                    k, sep, v = item.partition(b'=')
                    if sep and k.startswith(b'NEHUANG_'): env[k.decode()] = v.decode()
                found.append((int(entry.name), env))
            except (FileNotFoundError, ProcessLookupError): continue
        if len(found) != 1: raise RuntimeError('expected one existing service: '+name)
        return found[0]
    def alive(self, pid):
        try:
            os.kill(pid, 0)
            try: return pathlib.Path('/proc',str(pid),'stat').read_text().split(') ')[1][0] != 'Z'
            except FileNotFoundError: return False
        except ProcessLookupError: return False
    def stop(self, pid):
        if not self.alive(pid): return
        os.kill(pid, signal.SIGTERM)
        deadline=time.monotonic()+self.timeout
        while self.alive(pid) and time.monotonic()<deadline: time.sleep(.1)
        if self.alive(pid): raise RuntimeError('service did not stop gracefully; no forced kill')
    def start(self, name, log, env):
        with open(self.root/log,'ab',buffering=0) as output:
            p=subprocess.Popen([str(self.root/name)],cwd=self.root,env=env,stdin=subprocess.DEVNULL,stdout=output,stderr=output,start_new_session=True)
        self.new.append(p.pid)
        return p.pid
    def ready(self, pid, base):
        deadline=time.monotonic()+self.timeout
        while time.monotonic()<deadline:
            if not self.alive(pid): raise RuntimeError('new service exited; logs retained')
            try:
                for path in ('/healthz','/readyz'):
                    with urllib.request.urlopen(base+path,timeout=1) as r:
                        if r.status!=200: raise RuntimeError('not ready')
                return
            except Exception: time.sleep(.2)
        raise RuntimeError('service readiness timed out')
    def run(self):
        for name,_,_ in self.services:
            if not (self.build/name).is_file(): raise RuntimeError('build output missing: '+name)
            self.old[name] = self.discover(name)
        self.backup=self.root/'.deploy-backups'/str(int(time.time()*1000000))
        self.backup.mkdir(parents=True,mode=0o700)
        for name,_,_ in self.services: shutil.copy2(self.root/name,self.backup/name)
        stopped=set()
        try:
            for name,_,_ in reversed(self.services):
                self.stop(self.old[name][0]); stopped.add(name)
            for name,log,base in self.services:
                shutil.copy2(self.build/name,self.root/(name+'.next'))
                os.replace(self.root/(name+'.next'),self.root/name)
                pid=self.start(name,log,self.old[name][1]); self.ready(pid,base)
                (self.root/(name+'.pid')).write_text(str(pid)+'\n')
        except BaseException:
            rollback_ok=True
            for pid in reversed(self.new):
                try: self.stop(pid)
                except Exception: rollback_ok=False
            # Never overwrite a running replacement executable or start a duplicate.
            if rollback_ok:
                for name,log,base in self.services:
                    if name not in stopped: continue
                    shutil.copy2(self.backup/name,self.root/(name+'.next'))
                    os.replace(self.root/(name+'.next'),self.root/name)
                    try:
                        pid=self.start(name,log,self.old[name][1]); self.ready(pid,base)
                        (self.root/(name+'.pid')).write_text(str(pid)+'\n')
                    except Exception: rollback_ok=False
            print('Deployment failed; rollback '+('healthy' if rollback_ok else 'requires operator attention')+'; backup: '+str(self.backup),file=sys.stderr)
            raise
        print(json.dumps({'deployed':True,'backup':str(self.backup),'services':[s[0] for s in self.services]}))

if __name__=='__main__':
    def interrupted(*_): raise RuntimeError('deployment interrupted')
    signal.signal(signal.SIGTERM,interrupted); signal.signal(signal.SIGINT,interrupted)
    try: Deployment(sys.argv[1],sys.argv[2]).run()
    except BaseException as e:
        print('Deployment stopped: '+str(e),file=sys.stderr); sys.exit(1)
