package web

// indexPage 首页：三端入口。
const indexPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>qrcd 三端</title>
<style>body{font-family:system-ui,sans-serif;max-width:680px;margin:40px auto;padding:0 20px;color:#222}
a{display:block;padding:20px;margin:12px 0;border:2px solid #2a7;border-radius:12px;text-decoration:none;color:#2a7;font-size:18px}
a:hover{background:#2a7;color:#fff}
small{color:#888}</style></head>
<body>
<h1>qrcd · 二维码数据传输</h1>
<p>三个产物，纯光学（屏幕二维码 ↔ 摄像头），全程不联网：</p>
<a href="/sender">① 发送端 —— 选文件，屏幕播放二维码</a>
<a href="/relay">② 手机中继 —— 扫码存下，再重放给接收端</a>
<a href="/receiver">③ 接收端 —— 扫码还原并下载文件</a>
<hr><small>气隙传输：发送端电脑 →（手机扫码中转）→ 接收端电脑。</small>
</body></html>`

// senderPage 发送端：选文件→提交→屏幕逐帧播放 QR。
const senderPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>qrcd 发送端</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;margin:0;padding:20px;background:#111;color:#eee}
h1{font-size:20px} #drop{border:2px dashed #6a6;border-radius:12px;padding:40px;margin:20px auto;max-width:480px;cursor:pointer}
#drop.hover{background:#1a3a1a} #qr{margin:20px auto;max-width:90vw;image-rendering:pixelated;background:#fff}
.bar{background:#333;height:8px;border-radius:4px;margin:12px auto;max-width:400px}
.bar>i{display:block;height:100%;width:0;background:#5d9;border-radius:4px}
input[type=range]{width:240px} button{font-size:16px;padding:10px 24px;border:none;border-radius:8px;background:#2a7;color:#fff;cursor:pointer;margin:6px}
</style></head>
<body>
<h1>qrcd 发送端</h1>
<p>选文件，屏幕逐帧播放二维码，用另一台设备摄像头扫。</p>
<div id="drop">点击或拖入文件<input type="file" id="file" hidden></div>
<div><label>FPS <input type="range" id="fps" min="1" max="20" value="6"><span id="fpsv">6</span></label></div>
<button id="send">开始发送</button>
<button id="stop" hidden>停止</button>
<div class="bar"><i id="prog"></i></div>
<canvas id="qr"></canvas>
<div id="info"></div>
<script>
const $=id=>document.getElementById(id);
let frames=0,name='',size=0,playing=false,i=0,timer=null;
$('fps').oninput=e=>$('fpsv').textContent=e.target.value;
$('drop').onclick=()=>$('file').click();
['dragover'].forEach(e=>$('drop').addEventListener(e,ev=>{ev.preventDefault();$('drop').classList.add('hover')}));
['dragleave','drop'].forEach(e=>$('drop').addEventListener(e,ev=>{ev.preventDefault();$('drop').classList.remove('hover')}));
$('drop').addEventListener('drop',ev=>{if(ev.dataTransfer.files[0])$('file').files=ev.dataTransfer.files});
$('file').onchange=e=>{const f=e.target.files[0];if(f)$('drop').textContent='已选: '+f.name};
$('send').onclick=async()=>{
  const fd=new FormData();const f=$('file').files[0];
  if(f)fd.append('file',f); else fd.append('text',prompt('输入要发送的文本','')||'');
  $('info').textContent='编码中…';
  const r=await fetch('/api/encode',{method:'POST',body:fd});
  const j=await r.json();
  if(!r.ok){$('info').textContent='错误';return}
  frames=j.frames;name=j.name;size=j.size;i=0;playing=true;
  $('send').hidden=true;$('stop').hidden=false;
  $('info').textContent='文件: '+name+' ('+size+'B), 共 '+frames+' 帧';
  play();
};
$('stop').onclick=()=>{playing=false;clearTimeout(timer);$('send').hidden=false;$('stop').hidden=true;$('info').textContent='已停止'};
function play(){
  if(!playing)return;
  if(i>=frames){$('info').textContent='发送完成 ✓（循环重放中）';i=0}
  const img=new Image();
  img.onload=()=>{const c=$('qr');const sc=8;c.width=img.width*sc;c.height=img.height*sc;
    const x=c.getContext('2d');x.imageSmoothingEnabled=false;x.drawImage(img,0,0,c.width,c.height);
    $('prog').style.width=(100*i/frames)+'%'};
  img.src='/api/frame/'+i;i++;
  timer=setTimeout(play,1000/parseInt($('fps').value));
}
</script></body></html>`

// relayPage 手机中继：扫发送端 QR 逐帧存图→切重放，对准接收端摄像头播放。
// 图像存转发：存扫描到的 QR 帧快照，重放按序回放，手机无需理解协议。
const relayPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1">
<title>qrcd 手机中继</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;margin:0;padding:12px;background:#111;color:#eee}
h1{font-size:18px} video,#out{max-width:96vw;width:100%;border-radius:8px;background:#000}
video.hidden,#out.hidden{display:none}
.bar{background:#333;height:8px;border-radius:4px;margin:8px auto;max-width:400px}
.bar>i{display:block;height:100%;width:0;background:#5d9;border-radius:4px}
button{font-size:16px;padding:12px 24px;border:none;border-radius:8px;background:#2a7;color:#fff;cursor:pointer;margin:6px}
button:disabled{background:#555} .mode{font-size:14px;color:#9cf;margin:6px}
#lib{font-size:11px;color:#666} #info{min-height:24px}
</style></head>
<body>
<h1>qrcd 手机中继</h1>
<div class="mode" id="mode">① 扫码接收：对准发送端二维码</div>
<video id="cam" autoplay playsinline muted></video>
<canvas id="out" class="hidden"></canvas>
<div class="bar"><i id="prog"></i></div>
<div id="info">正在初始化摄像头…</div>
<button id="toggle">切换到 ② 重放模式</button>
<button id="clear">清空</button>
<div id="lib">QR 解码库加载中…</div>
<script src="https://cdn.jsdelivr.net/npm/jsQR@1.4.0/dist/jsQR.js"></script>
<script>
const $=id=>document.getElementById(id);
const video=$('cam'),out=$('out'),cx=out.getContext('2d');
let mode='scan',snaps=[],seen=new Set(),scanning=false,replaying=false,t=null;
function waitLib(){return typeof jsQR!=='undefined'?Promise.resolve():new Promise(r=>setTimeout(()=>waitLib().then(r),200))}
waitLib().then(()=>{$('lib').textContent=typeof jsQR==='undefined'?'⚠ jsQR 加载失败（需联网）':'QR 解码库就绪 ✓'});
async function startScan(){
  const stream=await navigator.mediaDevices.getUserMedia({video:{facingMode:'environment'}});
  video.srcObject=stream;await video.play();scanning=true;loop();
}
function loop(){
  if(!scanning||mode!=='scan')return;
  if(video.readyState>=2){
    out.width=video.videoWidth;out.height=video.videoHeight;cx.drawImage(video,0,0,out.width,out.height);
    const img=cx.getImageData(0,0,out.width,out.height);
    const r=jsQR(img.data,img.width,img.height,{inversionAttempts:'dontInvert'});
    if(r&&r.data){
      // 用解码内容做去重 key，避免同一帧重复存
      const key=r.data.length+':'+r.data.slice(0,32);
      if(!seen.has(key)){seen.add(key);snaps.push(cx.getImageData(0,0,out.width,out.height));
        $('info').textContent='已捕获 '+snaps.length+' 帧二维码';
        $('prog').style.width=Math.min(100,snaps.length*3)+'%';
        cx.strokeStyle='#5d9';cx.lineWidth=8;cx.beginPath();
        const L=r.location;cx.moveTo(L.topLeftCorner.x,L.topLeftCorner.y);cx.lineTo(L.topRightCorner.x,L.topRightCorner.y);
        cx.lineTo(L.bottomRightCorner.x,L.bottomRightCorner.y);cx.lineTo(L.bottomLeftCorner.x,L.bottomLeftCorner.y);cx.closePath();cx.stroke();
        out.classList.remove('hidden');video.classList.add('hidden');
        setTimeout(()=>{if(mode==='scan'){out.classList.add('hidden');video.classList.remove('hidden')}},100);
      }
    }
  }
  requestAnimationFrame(loop);
}
function startReplay(){
  scanning=false;video.srcObject&&video.srcObject.getTracks().forEach(t=>t.stop());
  video.classList.add('hidden');out.classList.remove('hidden');
  replaying=true;let k=0;
  (function next(){
    if(!replaying||k>=snaps.length){replaying=false;$('info').textContent='重放完成 ✓（再点可重放）';return}
    out.width=snaps[k].width;out.height=snaps[k].height;cx.putImageData(snaps[k],0,0);
    $('info').textContent='重放 '+(k+1)+'/'+snaps.length;
    $('prog').style.width=(100*(k+1)/snaps.length)+'%';k++;t=setTimeout(next,150);
  })();
}
$('toggle').onclick=()=>{
  if(mode==='scan'){mode='replay';replaying=false;clearTimeout(t);
    $('mode').textContent='② 重放：对准接收端摄像头';$('toggle').textContent='切回 ① 扫码接收';startReplay()}
  else{mode='scan';replaying=false;clearTimeout(t);
    $('mode').textContent='① 扫码接收：对准发送端二维码';$('toggle').textContent='切换到 ② 重放模式';startScan()}
};
$('clear').onclick=()=>{snaps=[];seen.clear();$('info').textContent='已清空';$('prog').style.width=0};
startScan().catch(e=>$('info').textContent='摄像头错误: '+e.message);
</script></body></html>`

// receiverPage 接收端：扫码→解析帧协议（32B 头+CRC32+元数据+按 seq 重组）→SHA-256 校验→下载。
// 系统源符号按序发送，收齐 0..BlockCount-1 即可重组（无需 Raptor 解码）；丢帧时提示重扫。
const receiverPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1">
<title>qrcd 接收端</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;margin:0;padding:12px;background:#111;color:#eee}
h1{font-size:18px} video{max-width:96vw;width:100%;border-radius:8px;background:#000}
video.hidden{display:none} canvas{display:none}
.bar{background:#333;height:8px;border-radius:4px;margin:8px auto;max-width:400px}
.bar>i{display:block;height:100%;width:0;background:#5d9;border-radius:4px}
button{font-size:16px;padding:12px 24px;border:none;border-radius:8px;background:#2a7;color:#fff;cursor:pointer;margin:6px}
#info{min-height:48px} #lib{font-size:11px;color:#666}
</style></head>
<body>
<h1>qrcd 接收端</h1>
<video id="cam" autoplay playsinline muted></video>
<canvas id="cv"></canvas>
<div class="bar"><i id="prog"></i></div>
<div id="info">对准二维码…</div>
<button id="save" hidden>下载文件</button>
<button id="stop">停止</button>
<div id="lib">QR 解码库加载中…</div>
<script src="https://cdn.jsdelivr.net/npm/jsQR@1.4.0/dist/jsQR.js"></script>
<script>
const $=id=>document.getElementById(id);
const video=$('cam'),cv=$('cv'),cx=cv.getContext('2d');
let running=true,meta=null,blocks={},tid='';
function waitLib(){return typeof jsQR!=='undefined'?Promise.resolve():new Promise(r=>setTimeout(()=>waitLib().then(r),200))}
waitLib().then(()=>{$('lib').textContent=typeof jsQR==='undefined'?'⚠ jsQR 加载失败':'QR 解码库就绪 ✓'});
// CRC32 IEEE
const crcTable=(()=>{let t=[];for(let n=0;n<256;n++){let c=n;for(let k=0;k<8;k++)c=c&1?0xEDB88320^(c>>>1):c>>>1;t[n]=c>>>0}return t})();
function crc32(buf){let c=0xFFFFFFFF;for(let i=0;i<buf.length;i++)c=crcTable[(c^buf[i])&0xFF]^(c>>>8);return (c^0xFFFFFFFF)>>>0}
function b2u32(b,o){return((b[o]<<24)|(b[o+1]<<16)|(b[o+2]<<8)|b[o+3])>>>0}
function b2u16(b,o){return((b[o]<<8)|b[o+1])>>>0}
async function sha256hex(buf){const d=await crypto.subtle.digest('SHA-256',buf);return[...new Uint8Array(d)].map(b=>b.toString(16).padStart(2,'0')).join('')}
function decStrToBytes(s){
  // 发送端用 string(data) 入码；QR byte 模式存原始字节。jsQR 返回 UTF-16 字符串，
  // 对二进制帧用 Latin1（每 char→1 字节）还原，与 ISO-8859-1 一致。
  const a=new Uint8Array(s.length);for(let i=0;i<s.length;i++)a[i]=s.charCodeAt(i)&0xFF;return a}
function handleFrame(bytes){
  if(bytes.length<36)return; // 32 头 + 4 CRC
  if(String.fromCharCode(...bytes.slice(0,4))!=='QRCD')return;
  const type=bytes[5],seq=b2u32(bytes,24),len=b2u32(bytes,28);
  if(32+len+4!==bytes.length)return;
  const payload=bytes.slice(32,32+len);
  const crc=b2u32(bytes,32+len);
  const calc=crc32(new Uint8Array([...bytes.slice(0,32),...payload]));
  if(crc!==calc)return; // CRC 校验失败丢弃
  if(type===1){ // 元数据
    const m=JSON.parse(new TextDecoder().decode(payload));
    meta=m;tid=Array.from(bytes.slice(8,24)).map(b=>b.toString(16).padStart(2,'0')).join('');
    $('info').textContent='收到元数据: '+m.name+' ('+m.size+'B, 需 '+m.blockCount+' 块)';
  }else if(type===2){ // 数据
    if(blocks[seq])return; blocks[seq]=payload;
    const got=Object.keys(blocks).length;
    if(meta){const pct=Math.min(100,got/meta.blockCount*100);$('prog').style.width=pct+'%';
      $('info').textContent='已收 '+got+'/'+meta.blockCount+' 块';
      if(got>=meta.blockCount)assemble()}
  }
}
function assemble(){
  if(!meta)return;const keys=Object.keys(blocks).map(Number).sort((a,b)=>a-b);
  let parts=[];for(const k of keys)if(k<meta.blockCount)parts.push(blocks[k]);
  let total=parts.reduce((s,p)=>s+p.length,0);
  let all=new Uint8Array(total);let o=0;for(const p of parts){all.set(p,o);o+=p.length}
  all=all.slice(0,meta.size); // 截断到原始大小
  sha256hex(all).then(h=>{if(h===meta.hash){
    $('info').textContent='✓ 还原成功，SHA-256 校验通过 ('+all.length+'B)';
    const b=new Blob([all]);const a=document.createElement('a');
    a.href=URL.createObjectURL(b);a.download=meta.name;a.click();running=false;
    $('save').hidden=false;$('save').onclick=()=>a.click();
  }else{$('info').textContent='✗ SHA-256 不匹配，请重扫（哈希 '+h.slice(0,12)+'…）';blocks={}}});
}
navigator.mediaDevices.getUserMedia({video:{facingMode:'environment'}}).then(s=>{video.srcObject=s;return video.play()}).then(loop).catch(e=>$('info').textContent='摄像头错误: '+e.message);
function loop(){
  if(!running)return;
  if(video.readyState>=2){
    cv.width=video.videoWidth;cv.height=video.videoHeight;cx.drawImage(video,0,0,cv.width,cv.height);
    const img=cx.getImageData(0,0,cv.width,cv.height);
    const r=jsQR(img.data,img.width,img.height,{inversionAttempts:'dontInvert'});
    if(r&&r.data){try{handleFrame(decStrToBytes(r.data))}catch(e){}}
  }
  requestAnimationFrame(loop);
}
$('stop').onclick=()=>{running=false;$('info').textContent='已停止'};
</script></body></html>`
