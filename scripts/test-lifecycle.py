#!/usr/bin/env python3
"""Exercise removal in a temporary root and curl|bash selection through a PTY.
Service managers and elevation are fixtures; never changes a real installation.
"""
from pathlib import Path
import argparse,os,pty,select,shlex,subprocess,tempfile,time
root=Path(__file__).resolve().parents[1]
parser=argparse.ArgumentParser();parser.add_argument('--packages',type=Path,required=True);args=parser.parse_args()
with tempfile.TemporaryDirectory(prefix='uscn-lifecycle-') as scratch:
 work=Path(scratch);tools=work/'tools';tools.mkdir();log=work/'calls';log.touch()
 def tool(name,code):
  p=tools/name;p.write_text(code);p.chmod(0o755);return p
 tool('uname','#!/bin/sh\ncase "$1" in -s) echo "$TEST_OS";; -m) echo arm64;; esac\n')
 tool('id','#!/bin/sh\necho "${TEST_UID:-0}"\n')
 manager='''#!/usr/bin/env python3
import os,sys
from pathlib import Path
args=sys.argv[1:];active=Path(os.environ['TEST_ACTIVE']);name=Path(sys.argv[0]).name
with open(os.environ['TEST_LOG'],'a') as f:f.write(name+' '+' '.join(args)+'\\n')
if args[0] in ('print','is-active'):sys.exit(0 if active.exists() else 1)
if args[0] in ('bootout','stop'):
 if os.environ.get('TEST_STOP_FAIL')=='1':sys.exit(39)
 if active.exists():active.unlink()
'''
 tool('launchctl',manager);tool('systemctl',manager)
 tool('PlistBuddy','''#!/usr/bin/env python3
import os,sys
print({'Print :Label':'si.uscn.gpu-agent','Print :UserName':'_uscn_gpu','Print :ProgramArguments:0':os.environ['TEST_BINARY']}[sys.argv[2]])
''')
 for system in ['Linux','Darwin']:
  base=work/system;base.mkdir();prefix=base/'usr/local';binary=prefix/'libexec/uscn-gpu-agent/uscn-gpu-agent';binary.parent.mkdir(parents=True);binary.write_text('FIXTURE_BINARY')
  cli=prefix/'bin/uscn-gpu-agent';cli.parent.mkdir();cli.symlink_to(binary)
  state=base/('var/db/uscn-gpu-agent' if system=='Darwin' else 'var/lib/uscn-gpu-agent');state.mkdir(parents=True,mode=0o700);(state/'device.json').write_text('PRIVATE_FIXTURE_IDENTITY');(state/'queue').mkdir();(state/'queue/report').write_text('PENDING_FIXTURE_REPORT')
  unit=base/('Library/LaunchDaemons/si.uscn.gpu-agent.plist' if system=='Darwin' else 'etc/systemd/system/uscn-gpu-agent.service');unit.parent.mkdir(parents=True)
  contents='PLIST_FIXTURE' if system=='Darwin' else 'User=uscn-gpu\nExecStart='+str(binary)+' service --config '+str(state/'device.json')+'\n'
  unit.write_text(contents);active=base/'active';active.touch()
  source=(root/'packaging/install.sh').read_text()
  for old in ['/usr/local','/var/lib/uscn-gpu-agent','/var/db/uscn-gpu-agent','/etc/systemd/system/uscn-gpu-agent.service','/Library/LaunchDaemons/si.uscn.gpu-agent.plist']:
   source=source.replace(old,str(base)+old)
  for old in ['/bin/launchctl','/usr/libexec/PlistBuddy']:source=source.replace(old,str(tools/Path(old).name))
  script=base/'install.sh';script.write_text(source)
  env=dict(os.environ,PATH=str(tools)+os.pathsep+os.environ['PATH'],TEST_OS=system,TEST_LOG=str(log),TEST_ACTIVE=str(active),TEST_BINARY=str(binary))
  def run(extra=None):return subprocess.run(['bash',str(script),'--uninstall'],env=dict(env,**(extra or {})),capture_output=True,text=True)
  r=run({'TEST_STOP_FAIL':'1'});assert r.returncode!=0 and binary.exists() and unit.exists() and active.exists(),(system,r.stderr)
  cli.unlink();cli.write_text('UNRELATED_CLI');keep=binary.parent/'keep.txt';keep.write_text('UNRELATED_FILE')
  r=run();assert r.returncode==0,(system,r.stderr);assert not binary.exists() and not unit.exists() and not active.exists()
  assert cli.read_text()=='UNRELATED_CLI' and keep.read_text()=='UNRELATED_FILE'
  assert (state/'device.json').read_text()=='PRIVATE_FIXTURE_IDENTITY' and (state/'queue/report').read_text()=='PENDING_FIXTURE_REPORT'
  r=run();assert r.returncode==0,'repeat uninstall failed'
  cli.unlink();cli.symlink_to(binary);r=run();assert r.returncode==0 and not cli.is_symlink()
  outside=base/'outside';outside.write_text('KEEP');binary.symlink_to(outside);r=run();assert r.returncode!=0 and outside.read_text()=='KEEP';binary.unlink()
  if system=='Linux':
   binary.write_text('FIXTURE_BINARY');unit.write_text('User=someone_else\nExecStart=/unrelated\n');r=run();assert r.returncode!=0 and binary.exists() and unit.exists()
 tool('sudo','#!/bin/sh\nprintf "sudo %s\\n" "$*" >> "$TEST_LOG"\nexit 77\n')
 tool('curl','''#!/usr/bin/env python3
import os,sys,shutil
from pathlib import Path
args=sys.argv[1:];url=next(a for a in args if a.startswith('https://'))
with open(os.environ['TEST_LOG'],'a')as f:f.write('download '+url+'\\n')
shutil.copyfile(Path(os.environ['TEST_PACKAGES'])/url.rsplit('/',1)[1],args[args.index('-o')+1])
''')
 env=dict(os.environ,PATH=str(tools)+os.pathsep+os.environ['PATH'],TEST_OS='Linux',TEST_UID='501',TEST_LOG=str(log),TEST_PACKAGES=str(args.packages.resolve()),TMPDIR=str(work))
 def terminal(choice):
  pid,fd=pty.fork()
  if pid==0:os.execvpe('bash',['bash','-c','cat '+shlex.quote(str(root/'install.sh'))+' | bash'],env)
  output=b'';sent=False;deadline=time.monotonic()+15
  try:
   while time.monotonic()<deadline:
    ready,_,_=select.select([fd],[],[],.1)
    if ready:
     try:data=os.read(fd,65536)
     except OSError:break
     if not data:break
     output+=data
     if not sent and '请选择'.encode() in output:os.write(fd,(choice+'\n').encode());sent=True
   else:os.kill(pid,9);raise AssertionError('menu blocked')
  finally:os.close(fd)
  _,status=os.waitpid(pid,0);return os.waitstatus_to_exitcode(status),output.decode(errors='replace')
 for choice in ['0','','9']:
  before=log.read_text();code,out=terminal(choice);assert code==(2 if choice=='9' else 0),(code,out);assert log.read_text()==before,'cancel/invalid menu touched network or privilege'
 for choice,action in [('1','--install'),('2','--uninstall')]:
  code,out=terminal(choice);assert code==77,(code,out);assert action in log.read_text().splitlines()[-1];assert not list(work.glob('uscn-gpu-install.*')),'operation left scratch'
 for flags in [['--install','--uninstall'],['--download-only','--uninstall']]:
  result=subprocess.run(['bash',str(root/'install.sh'),*flags],env=env,capture_output=True);assert result.returncode==2
print('PASS_UNIX_MENU_PIPE_TTY_ACTION_FORWARD_CANCEL_REPEAT_UNINSTALL_STOP_FAILURE_STATE_AND_UNRELATED_FILES_PRESERVED')
print('Windows service removal requires a Windows host for native execution.')
