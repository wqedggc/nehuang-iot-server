// Real loopback HTTP; shorten the 10s budget to 100ms to test body cancellation quickly.
const http=require('node:http'),fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),path=require('node:path');
(async()=>{
 const server=http.createServer((req,res)=>{res.writeHead(200,{'Content-Type':'application/json'});res.flushHeaders();const timer=setTimeout(()=>res.end('{"data":[]}'),500);res.on('close',()=>clearTimeout(timer));});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const sandbox={fetch,AbortController,setTimeout:(fn,ms)=>setTimeout(fn,ms===10000?100:ms),clearTimeout,location:{origin:`http://127.0.0.1:${server.address().port}`,protocol:'http:'},localStorage:{getItem:()=>null,removeItem:()=>{}},window:null};sandbox.window=sandbox;
 vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../static/shared/api-client.js'),'utf8'),sandbox);
 try{const start=Date.now();await assert.rejects(sandbox.NHApi.getDashboard('1d'),e=>e.message.includes('请求超时')&&e.status===0);assert(Date.now()-start<450);console.log('response-body deadline: PASS');}
 finally{server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
})().catch(e=>{console.error(e);process.exitCode=1;});
