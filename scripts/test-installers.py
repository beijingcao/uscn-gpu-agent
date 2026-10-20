#!/usr/bin/env python3
"""Exercise real archive verification and extraction without installing a service.
Requires an already built package directory. Network and privilege tools are
replaced by controlled fixtures; no GPU or production account is used.
"""
from pathlib import Path
import argparse, hashlib, json, os, shutil, stat, subprocess, tempfile
root=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser();p.add_argument('--packages',type=Path,default=root/'dist/release');a=p.parse_args();packages=a.packages.resolve()
manifest=json.loads((packages/'BUILD.json').read_text())
for item in manifest['packages']:
 assert hashlib.sha256((packages/item['file']).read_bytes()).hexdigest()==item['sha256']
for path in [root/'install.sh',root/'packaging/install.sh',root/'packaging/install.command']:
 subprocess.run(['bash','-n',str(path)],check=True)
assert (root/'install.ps1').read_bytes().startswith(b'\xef\xbb\xbf')
assert (root/'packaging/install.ps1').read_bytes().startswith(b'\xef\xbb\xbf')
with tempfile.TemporaryDirectory(prefix='uscn-installer-tests-') as scratch:
 work=Path(scratch);binary=work/'bin';binary.mkdir();log=work/'calls'
 uname=binary/'uname';uname.write_text('#!/bin/sh\ncase "$1" in -s) echo "$TEST_UNAME_S";; -m) echo "$TEST_UNAME_M";; *) exit 99;; esac\n');uname.chmod(0o755)
 curl=binary/'curl';curl.write_text('''#!/usr/bin/env python3
import os,shutil,sys
from pathlib import Path
args=sys.argv[1:];target=Path(args[args.index('-o')+1]);urls=[a for a in args if a.startswith('https://')];assert len(urls)==1
url=urls[0];assert url.startswith(os.environ['TEST_EXPECTED_BASE']+'/')
with open(os.environ['TEST_CALL_LOG'],'a')as f:f.write('download '+url+'\\n')
if os.environ.get('TEST_DOWNLOAD_MODE')=='error':sys.exit(22)
if os.environ.get('TEST_DOWNLOAD_MODE')=='corrupt':target.write_bytes(b'CORRUPTED_FIXTURE')
else:shutil.copyfile(Path(os.environ['TEST_PACKAGES'])/url.rsplit('/',1)[1],target)
''');curl.chmod(0o755)
 sudo=binary/'sudo';sudo.write_text('#!/bin/sh\nprintf "PRIVILEGE_CALLED\\n" >> "$TEST_CALL_LOG"\nexit 77\n');sudo.chmod(0o755)
 def run(osname='Linux',arch='x86_64',args=('--download-only',),mode='ok',base='https://uscn.si/downloads/gpu-agent/v1.0.2'):
  env=os.environ.copy();env.update(PATH=str(binary)+os.pathsep+env.get('PATH',''),TMPDIR=str(work),TEST_UNAME_S=osname,TEST_UNAME_M=arch,TEST_PACKAGES=str(packages),TEST_CALL_LOG=str(log),TEST_EXPECTED_BASE=base,TEST_DOWNLOAD_MODE=mode)
  return subprocess.run(['bash',str(root/'install.sh'),*args],env=env,text=True,capture_output=True)
 for system,arch in [('Linux','x86_64'),('Linux','aarch64'),('Linux','arm64'),('Darwin','arm64'),('Darwin','x86_64')]:
  result=run(system,arch);assert result.returncode==0,(system,result.stderr);assert 'Verified USCN GPU Agent 1.0.2:'in result.stdout
  package=Path(result.stdout.strip().split(': ',1)[1]);assert package.is_dir()and(package/'SHA256SUMS').is_file();shutil.rmtree(package.parent)
 result=run(args=('--download-only','--github'),base='https://github.com/beijingcao/uscn-gpu-agent/releases/download/v1.0.2');assert result.returncode==0,result.stderr;shutil.rmtree(Path(result.stdout.strip().split(': ',1)[1]).parent)
 before=log.read_text();result=run(arch='ppc64le');assert result.returncode!=0 and log.read_text()==before
 result=run(args=('--unknown',));assert result.returncode==2 and log.read_text()==before
 result=run(mode='corrupt');assert result.returncode!=0 and 'SHA256 mismatch'in result.stderr;assert not list(work.glob('uscn-gpu-install.*')),'failed verification left download scratch'
 result=run(mode='error');assert result.returncode!=0 and not list(work.glob('uscn-gpu-install.*'))
 assert 'PRIVILEGE_CALLED'not in log.read_text()
# Reproduce the reported macOS umask 077 failure without touching system paths.
# Execute the actual program-directory preparation command against a temp root.
with tempfile.TemporaryDirectory(prefix='uscn-mac-permissions-') as scratch:
 prefix=Path(scratch)/'usr/local';prefix.mkdir(parents=True)
 installer=(root/'packaging/install.sh').read_text()
 line=next(x.strip() for x in installer.splitlines() if x.strip().startswith('/usr/bin/install -d -m 755 /usr/local/libexec'))
 command=line.replace('/usr/local',str(prefix))
 subprocess.run(['bash','-c','umask 077\n'+command],check=True)
 for name in ['libexec','libexec/uscn-gpu-agent','bin']:
  assert stat.S_IMODE((prefix/name).stat().st_mode)==0o755, 'service account cannot traverse '+name
 state=Path(scratch)/'state';state.mkdir(mode=0o700)
 assert stat.S_IMODE(state.stat().st_mode)==0o700
print('PASS_INSTALLER_LINUX_MAC_INTEL_ARM_GITHUB_CORRUPTION_FAILURE_NO_PRIVILEGE_MAC_SERVICE_TRAVERSAL')
print('Windows script encoding/digest checked; PowerShell service execution requires a Windows host.')
