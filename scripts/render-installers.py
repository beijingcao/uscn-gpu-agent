#!/usr/bin/env python3
"""Pin bootstrap installers to the actual release archive SHA256 digests."""
from pathlib import Path
import argparse, hashlib, json
root=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser();p.add_argument('--packages',type=Path,required=True);args=p.parse_args()
manifest=json.loads((args.packages/'BUILD.json').read_text());assert manifest['version']=='1.0.3'
names={'LINUX_ARM64_SHA':'uscn-gpu-agent-linux-arm64.tar.gz','LINUX_SHA':'uscn-gpu-agent-linux-amd64.tar.gz','MAC_SHA':'uscn-gpu-agent-macos-universal.zip','WINDOWS_SHA':'uscn-gpu-agent-windows-amd64.zip'}
packages={v['file']:v for v in manifest['packages']};values={'VERSION':manifest['version']}
for k,n in names.items():
 path=args.packages/n;digest=hashlib.sha256(path.read_bytes()).hexdigest();assert digest==packages[n]['sha256'];values[k]=digest
for name in ['install.sh','install.ps1']:
 text=(root/'templates'/(name+'.in')).read_text()
 for key,value in values.items():text=text.replace('@'+key+'@',value)
 assert not any('@'+key+'@' in text for key in values)
 target=root/name;target.write_text(text,encoding='utf-8-sig' if name.endswith('.ps1') else 'utf-8');target.chmod(0o755 if name.endswith('.sh') else 0o644)
print('BOOTSTRAPS_PINNED_TO_VERIFIED_RELEASE_PACKAGES')
