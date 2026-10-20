#!/usr/bin/env python3
"""Build fixed v1.0.3 release packages from this standalone source. Requires macOS for Universal signing."""
from pathlib import Path
import argparse, hashlib, json, os, shutil, subprocess, tarfile, zipfile

parser=argparse.ArgumentParser()
parser.add_argument('--out',required=True)
args=parser.parse_args()
repo=Path(__file__).resolve().parents[1]
source=repo
out=Path(args.out).resolve()
out.mkdir(parents=True,exist_ok=False)
run=None
templates=repo/'packaging'
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def build(target,goos,arch):
    env=os.environ.copy();env.update(GOOS=goos,GOARCH=arch,CGO_ENABLED='0')
    subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w','-o',str(target),'./cmd/uscn-gpu-agent'],cwd=source,env=env,check=True)
    target.chmod(0o755)
build(out/'mac-arm64','darwin','arm64')
build(out/'mac-amd64','darwin','amd64')
build(out/'linux-amd64','linux','amd64')
build(out/'linux-arm64','linux','arm64')
build(out/'windows-amd64.exe','windows','amd64')
subprocess.run(['/usr/bin/lipo','-create',str(out/'mac-arm64'),str(out/'mac-amd64'),'-output',str(out/'mac-universal')],check=True)
subprocess.run(['/usr/bin/codesign','--force','--sign','-',str(out/'mac-universal')],check=True,stdout=subprocess.DEVNULL)
subprocess.run(['/usr/bin/codesign','--verify','--strict',str(out/'mac-universal')],check=True)
archs=subprocess.check_output(['/usr/bin/lipo','-archs',str(out/'mac-universal')],text=True).split()
if set(archs)!={'x86_64','arm64'}:raise RuntimeError('universal architecture mismatch')
packages=[]
for label,binary,files,archive in [
 ('linux-amd64','linux-amd64',['install.sh','README.txt'],'uscn-gpu-agent-linux-amd64.tar.gz'),
 ('linux-arm64','linux-arm64',['install.sh','README.txt'],'uscn-gpu-agent-linux-arm64.tar.gz'),
 ('windows-amd64','windows-amd64.exe',['install.ps1','README.txt'],'uscn-gpu-agent-windows-amd64.zip'),
 ('macos-universal','mac-universal',['install.sh','install.command','README.txt'],'uscn-gpu-agent-macos-universal.zip')]:
    package=out/('uscn-gpu-agent-'+label);package.mkdir()
    name='uscn-gpu-agent.exe' if label=='windows-amd64' else 'uscn-gpu-agent'
    shutil.copy2(out/binary,package/name)
    for file in files:shutil.copy2(templates/file,package/file)
    for p in package.iterdir():p.chmod(0o755 if p.name in ('uscn-gpu-agent','install.sh','install.command') else 0o644)
    (package/'SHA256SUMS').write_text(''.join(digest(p)+'  '+p.name+'\n' for p in sorted(package.iterdir()) if p.is_file()))
    path=out/archive
    if archive.endswith('.zip'):
        with zipfile.ZipFile(path,'w',zipfile.ZIP_DEFLATED) as z:
            for p in sorted(package.iterdir()):z.write(p,package.name+'/'+p.name)
        with zipfile.ZipFile(path) as z:
            if z.testzip() is not None:raise RuntimeError('archive integrity failed')
    else:
        with tarfile.open(path,'w:gz') as t:t.add(package,arcname=package.name)
    (out/(archive+'.sha256')).write_text(digest(path)+'  '+archive+'\n')
    packages.append({'file':archive,'sha256':digest(path),'bytes':path.stat().st_size})
(out/'BUILD.json').write_text(json.dumps({'version':'1.0.3','packages':packages,'mac_architectures':archs,'mac_signature':'ad-hoc','apple_notarization':False,'windows_code_signing':False,'real_nvidia_test':'PENDING_AUTHORIZED_HARDWARE','windows_runtime_test':'PENDING_AUTHORIZED_HOST'},indent=2)+'\n')
print(json.dumps({'built':len(packages),'version':'1.0.3','mac_architectures':archs}))
