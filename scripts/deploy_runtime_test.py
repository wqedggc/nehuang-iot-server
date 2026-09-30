import importlib.util, pathlib, tempfile, unittest, os, subprocess
spec=importlib.util.spec_from_file_location('deploy_runtime',pathlib.Path(__file__).with_name('deploy_runtime.py')); m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class SimulatedDeployment(m.Deployment):
    def __init__(self,root,build,fail=False):
        super().__init__(root,build);self.running={1:'nehuang-server',2:'gateway-adapter',3:'gateway-adapter-mock'};self.next=10;self.fail=fail
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
if __name__=='__main__':unittest.main()
