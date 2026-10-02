#!/usr/bin/env python3
import subprocess,json,time,statistics,math,re,urllib.request,argparse
parser=argparse.ArgumentParser(description="Read-only serial performance baseline; never clears data or sends commands")
parser.add_argument("--mysql-defaults-file",default="configs/mysql/nehuang_agent.cnf")
parser.add_argument("--samples",type=int,default=5,choices=range(1,6))
args=parser.parse_args()
queries={
'payload_count':"SELECT COUNT(*) FROM raw_device_events IGNORE INDEX(uk_raw_source_capture) WHERE source_type=1 AND received_at_ms>=NOW()-INTERVAL 1 DAY",
'payload_page_1':"SELECT e.raw_event_id FROM raw_device_events e JOIN (SELECT raw_event_id,received_at_ms FROM raw_device_events IGNORE INDEX(uk_raw_source_capture) WHERE source_type=1 AND received_at_ms>=NOW()-INTERVAL 1 DAY ORDER BY received_at_ms DESC,raw_event_id DESC LIMIT 20) page ON e.raw_event_id=page.raw_event_id ORDER BY page.received_at_ms DESC,page.raw_event_id DESC",
'dashboard_ingest':"SELECT COUNT(*),COALESCE(SUM(event_type=1),0),COALESCE(SUM(event_type=2),0),COALESCE(SUM(event_type=3),0),COALESCE(SUM(event_type=4),0) FROM raw_device_events WHERE received_at_ms>=NOW()-INTERVAL 1 DAY",
'dashboard_heartbeat':"SELECT COUNT(*),COALESCE(SUM(parse_status=2),0),COALESCE(SUM(parse_status=3),0) FROM raw_device_events WHERE received_at_ms>=NOW()-INTERVAL 1 DAY AND event_type=2"}
r={'sample_count':args.samples,'concurrency':1,'boundary':'MySQL client round-trip plus SQL; no API/render/physical command latency; warm repeated samples; P95 nearest-rank; small sample, not a production percentile estimate','queries':{}}
def plan_tables(value):
 result=[]
 if isinstance(value,dict):
  if 'table_name' in value:
   result.append({k:value[k] for k in ['table_name','access_type','key','used_key_parts','rows_examined_per_scan','rows_produced_per_join','using_index','filtered'] if k in value})
  for child in value.values():result.extend(plan_tables(child))
 elif isinstance(value,list):
  for child in value:result.extend(plan_tables(child))
 return result
for name,q in queries.items():
 plan=subprocess.run(['mysql','--defaults-extra-file='+args.mysql_defaults_file,'-N','-s','--raw','-e','EXPLAIN FORMAT=JSON '+q],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 plan_result=json.loads(plan.stdout.decode()) if plan.returncode==0 else {'error':'EXPLAIN failed'}
 samples=[];errors=[]
 for i in range(args.samples):
  start=time.perf_counter();p=subprocess.run(['mysql','--defaults-extra-file='+args.mysql_defaults_file,'-N','-s','-e','SET SESSION MAX_EXECUTION_TIME=5000; '+q],stdout=subprocess.DEVNULL,stderr=subprocess.PIPE);ms=round((time.perf_counter()-start)*1000,2)
  if p.returncode:errors.append({'sample':i,'error_number':re.findall(r'ERROR (\d+)',p.stderr.decode())})
  else:samples.append(ms)
  time.sleep(.2)
 r['queries'][name]={'plan':plan_result,'samples_ms':samples,'errors':errors,'p50_ms':statistics.median(samples) if samples else None,'p95_ms':sorted(samples)[math.ceil(.95*len(samples))-1] if samples else None}
try:
 d=json.load(urllib.request.urlopen('http://127.0.0.1:18081/healthz',timeout=3));r['adapter_observation']={k:d.get(k) for k in ['status','active_connections','raw_pending','raw_write_failures','forward_failures']}
except Exception:r['adapter_observation']='unavailable'
print(json.dumps(r))
