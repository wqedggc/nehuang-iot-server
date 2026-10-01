import importlib.util, pathlib, tempfile, unittest, os, subprocess, io, contextlib
from unittest import mock
spec=importlib.util.spec_from_file_location('deploy_runtime',pathlib.Path(__file__).with_name('deploy_runtime.py')); m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class SimulatedDeployment(m.Deployment):
    def __init__(self,root,build,fail=False):
        super().__init__(root,build);self.running={1:'nehuang-server',2:'gateway-adapter',3:'gateway-adapter-mock'};self.next=10;self.fail=fail
    def probe(self,base): return {"connection_shutdown_supported":True,"active_connections":1}
    def alive(self,pid): return pid in self.running
    def discover(self,name): return (1 if name=='nehuang-server' else 2,{'NEHUANG_TOKEN':'existing-only'})
    def stop(self,pid): self.running.pop(pid,None)
    def start(self,name,log,env):
        assert env=={'NEHUANG_TOKEN':'existing-only'}
        self.next+=1;self.running[self.next]=name;self.new.append(self.next);return self.next
    def ready(self,pid,base):
        name=self.running[pid]
        if self.fail and name=='gateway-adapter' and (self.root/name).read_text()=='new': raise RuntimeError('mock failed readiness')
class DeploymentTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.root=pathlib.Path(self.tmp.name);self.build=self.root/'build';self.build.mkdir()
        for name in ['nehuang-server','gateway-adapter']:(self.root/name).write_text('old');(self.build/name).write_text('new')
        (self.root/'.env').write_text('unchanged');(self.root/'journal').write_text('retain')
    def tearDown(self):self.tmp.cleanup()
    def test_success_preserves_mock_and_configuration(self):
        d=SimulatedDeployment(self.root,self.build);d.run();self.assertEqual(d.running[3],'gateway-adapter-mock');self.assertEqual((self.root/'.env').read_text(),'unchanged');self.assertEqual((self.root/'journal').read_text(),'retain');self.assertEqual((self.root/'nehuang-server').read_text(),'new')
    def test_adapter_failure_restores_both_original_binaries_and_services(self):
        d=SimulatedDeployment(self.root,self.build,True)
        with self.assertRaises(RuntimeError):d.run()
        self.assertCountEqual(d.running.values(),['nehuang-server','gateway-adapter','gateway-adapter-mock'])
        for name in ['nehuang-server','gateway-adapter']:self.assertEqual((self.root/name).read_text(),'old')
    def test_missing_build_never_stops_existing(self):
        (self.build/'gateway-adapter').unlink();d=SimulatedDeployment(self.root,self.build)
        with self.assertRaises(RuntimeError):d.run()
        self.assertEqual(d.running,{1:'nehuang-server',2:'gateway-adapter',3:'gateway-adapter-mock'})
    def test_stop_timeout_then_exit_restores_attempted_adapter_and_checks_both(self):
        class LateExit(SimulatedDeployment):
            def __init__(self,*args):super().__init__(*args);self.stop_calls=[];self.checked=[]
            def stop(self,pid):
                self.stop_calls.append(pid)
                if pid==2 and self.stop_calls.count(2)==1:
                    self.running.pop(pid,None)
                    raise RuntimeError('exit immediately after stop deadline')
                super().stop(pid)
            def ready(self,pid,base):self.checked.append(self.running.get(pid));super().ready(pid,base)
        d=LateExit(self.root,self.build);stderr=io.StringIO()
        with contextlib.redirect_stderr(stderr),self.assertRaises(RuntimeError):d.run()
        self.assertCountEqual(d.running.values(),['nehuang-server','gateway-adapter','gateway-adapter-mock'])
        self.assertCountEqual(d.checked,['nehuang-server','gateway-adapter'])
        self.assertIn('rollback healthy',stderr.getvalue())
    def test_stop_timeout_still_alive_never_claims_healthy_or_duplicates(self):
        class Hung(SimulatedDeployment):
            def stop(self,pid):
                if pid==2:raise RuntimeError('legacy peer still connected')
                super().stop(pid)
        d=Hung(self.root,self.build);stderr=io.StringIO()
        with contextlib.redirect_stderr(stderr),self.assertRaises(RuntimeError):d.run()
        self.assertNotIn('rollback healthy',stderr.getvalue())
        self.assertIn('requires operator attention',stderr.getvalue())
        self.assertEqual(d.running,{1:'nehuang-server',2:'gateway-adapter',3:'gateway-adapter-mock'})
        self.assertEqual(d.new,[])
    def test_legacy_active_connection_preflight_leaves_old_services_untouched(self):
        class Legacy(SimulatedDeployment):
            def probe(self,base):return {'active_connections':1}
        d=Legacy(self.root,self.build)
        with self.assertRaises(RuntimeError):d.run()
        self.assertEqual(d.running,{1:'nehuang-server',2:'gateway-adapter',3:'gateway-adapter-mock'})
        self.assertIsNone(d.backup)
    def test_unrestarted_http_is_also_verified_before_healthy(self):
        class BrokenHTTP(SimulatedDeployment):
            def stop(self,pid):
                if pid==2 and 2 in self.running:
                    self.running.pop(2);raise RuntimeError('late exit')
                super().stop(pid)
            def ready(self,pid,base):
                if self.running.get(pid)=='nehuang-server':raise RuntimeError('old HTTP unhealthy')
                super().ready(pid,base)
        d=BrokenHTTP(self.root,self.build);stderr=io.StringIO()
        with contextlib.redirect_stderr(stderr),self.assertRaises(RuntimeError):d.run()
        self.assertNotIn('rollback healthy',stderr.getvalue())
        self.assertIn('requires operator attention',stderr.getvalue())
    def test_pid_reuse_is_not_signalled(self):
        d=m.Deployment(self.root,self.build);d.identities[123]=('known','cwd','start1')
        with mock.patch.object(d,'alive',return_value=True),mock.patch.object(d,'process_identity',return_value=('other','cwd','start2')),mock.patch.object(m.os,'kill') as kill:
            with self.assertRaises(RuntimeError):d.stop(123)
            kill.assert_not_called()
if __name__=='__main__':unittest.main()
