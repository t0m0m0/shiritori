package srv

const resultPageHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>

<!-- OGP -->
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:image" content="%s">
<meta property="og:url" content="%s">
<meta property="og:type" content="website">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">

<!-- Twitter Card -->
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="%s">
<meta name="twitter:description" content="%s">
<meta name="twitter:image" content="%s">

<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@300;400;500;700;900&family=Zen+Antique&family=Shippori+Mincho:wght@400;700&family=Zen+Maru+Gothic:wght@400;500;700&display=swap" rel="stylesheet">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --primary:#c23a22;--primary-light:#d4604c;--primary-dark:#a12e18;
  --accent:#3d6b5e;--accent-light:#5a8f7e;
  --danger:#b83a2a;--success:#3d6b5e;
  --bg:#f5f0e8;--surface:#faf7f0;--surface2:#ede8dc;
  --text:#2c2420;--text2:#8a7e72;--text3:#c4b8a8;
  --radius:4px;--shadow:0 1px 4px rgba(44,36,32,.08);
  --border:#d8d0c4;
  --font-body:'Zen Maru Gothic','Hiragino Maru Gothic Pro',sans-serif;
  --font-head:'Shippori Mincho',serif;
  --font-display:'Zen Antique',serif;
}
body{
  font-family:var(--font-body);
  background:var(--bg);
  background-image:
    radial-gradient(ellipse at 20%% 50%%,rgba(194,58,34,.018) 0%%,transparent 70%%),
    radial-gradient(ellipse at 80%% 20%%,rgba(61,107,94,.018) 0%%,transparent 70%%);
  color:var(--text);
  min-height:100dvh;line-height:1.7;
  position:relative;
}
body::before{
  content:"";position:fixed;inset:0;pointer-events:none;z-index:0;
  background-image:url("data:image/svg+xml,%%3Csvg width='200' height='200' xmlns='http://www.w3.org/2000/svg'%%3E%%3Cfilter id='n'%%3E%%3CfeTurbulence baseFrequency='.7' numOctaves='4' stitchTiles='stitch'/%%3E%%3C/filter%%3E%%3Crect width='100%%25' height='100%%25' filter='url(%%23n)' opacity='.025'/%%3E%%3C/svg%%3E");
}
.header{
  text-align:center;padding:2.5rem 1rem 2rem;
  background:var(--bg);color:var(--text);
  position:relative;overflow:visible;
  border-bottom:1px solid var(--border);
}
.header::after{
  content:"";position:absolute;bottom:-1px;left:5%%;right:5%%;height:3px;
  background:linear-gradient(90deg,
    transparent 0%%,rgba(44,36,32,.12) 4%%,rgba(44,36,32,.25) 12%%,
    rgba(44,36,32,.3) 20%%,rgba(44,36,32,.15) 32%%,rgba(44,36,32,.3) 38%%,
    rgba(44,36,32,.28) 55%%,transparent 57%%,rgba(44,36,32,.3) 60%%,
    rgba(44,36,32,.25) 78%%,rgba(44,36,32,.1) 92%%,transparent 100%%);
  border-radius:2px;
}
.header h1{
  font-family:var(--font-head);
  font-size:2.8rem;font-weight:700;
  letter-spacing:.15em;color:var(--text);
}
.header a{color:inherit;text-decoration:none}
.header a:hover{opacity:.8}
.header p{
  font-size:.85rem;color:var(--text2);margin-top:.4rem;
  font-family:var(--font-head);letter-spacing:.1em;
}
.container{max-width:600px;margin:0 auto;padding:1.5rem 1rem;position:relative;z-index:1}
.card{
  background:var(--surface);
  border:1px solid var(--border);
  border-radius:var(--radius);
  padding:1.5rem;box-shadow:var(--shadow);margin-bottom:1rem;
}
.card h2{
  font-family:var(--font-head);
  font-size:1.1rem;margin-bottom:1rem;color:var(--text);
  padding-bottom:.5rem;
  border-bottom:1px solid var(--border);
  letter-spacing:.05em;
}
.card h2::before{
  content:"●";color:var(--primary);margin-right:.4rem;font-size:.6rem;
  vertical-align:middle;
}
.reason{
  text-align:center;font-size:1.1rem;font-weight:700;
  color:var(--primary);margin-bottom:.5rem;
  font-family:var(--font-head);letter-spacing:.05em;
}
.genre-tag{
  text-align:center;font-size:.85rem;color:var(--accent);
  margin-bottom:1rem;
}
.scores{list-style:none}
.score-item{
  display:flex;justify-content:space-between;align-items:center;
  padding:.6rem 1rem;border-radius:var(--radius);margin-bottom:.4rem;
  background:var(--surface2);border:1px solid var(--border);
}
.score-item:first-child{
  background:linear-gradient(90deg,#f5ebe0,#ede1d0);
  border-color:#d4c4a8;font-weight:700;
}
.score-rank{width:2rem;text-align:center;font-weight:700}
.score-name{flex:1;text-align:left;margin-left:.5rem}
.score-pts{font-weight:700;color:var(--primary)}
.chain-summary{
  font-size:.85rem;color:var(--text2);padding:.5rem .75rem;
  background:var(--surface2);border-radius:var(--radius);margin-bottom:.75rem;
  border:1px solid var(--border);
  word-break:break-all;
}
.history-list{list-style:none}
.history-list li{
  display:flex;align-items:center;gap:.5rem;
  padding:.35rem .5rem;border-bottom:1px solid var(--border);
  font-size:.85rem;
}
.history-list li:last-child{border-bottom:none}
.h-num{color:var(--text2);min-width:1.5rem;text-align:right;font-size:.75rem}
.h-word{font-weight:600;color:var(--primary-dark)}
.h-player{color:var(--text2);font-size:.75rem;margin-left:auto}
.cta{text-align:center;margin-top:1.5rem}
.btn{
  display:inline-block;padding:.7rem 2.5rem;
  border-radius:var(--radius);
  font-weight:700;font-size:.95rem;text-decoration:none;
  font-family:var(--font-body);
  background:var(--primary);color:#fff;
  border:1px solid var(--primary-dark);
  transition:background .2s;
  letter-spacing:.05em;
}
.btn:hover{background:var(--primary-dark)}
.footer{
  text-align:center;padding:2rem;color:var(--text3);
  font-size:.8rem;font-family:var(--font-head);letter-spacing:.1em;
}
</style>
</head>
<body>
<div class="header">
  <h1><a href="/">し り と り</a></h1>
  <p>ことばを繋ぐ、みんなで遊ぶ</p>
</div>
<div class="container">
  <div class="card">
    <p class="reason" id="reason"></p>
    <p class="genre-tag" id="genreTag"></p>
    <h2>スコア</h2>
    <ul class="scores" id="scores"></ul>
  </div>
  <div class="card">
    <h2>しりとりチェーン</h2>
    <div class="chain-summary" id="chain"></div>
    <ul class="history-list" id="history"></ul>
  </div>
  <div class="cta">
    <a class="btn" href="/">しりとりで遊ぶ</a>
  </div>
</div>
<div class="footer">し り と り — マルチプレイヤー</div>
<script>
const result = %s;
const medals = ['🥇','🥈','🥉'];

// Reason
let reason = result.reason || '';
if (result.winner) reason = result.winner + ' さんの勝利！';
document.getElementById('reason').textContent = reason;

// Genre
if (result.genre && result.genre !== 'なし') {
  document.getElementById('genreTag').textContent = 'ジャンル: ' + result.genre;
}

// Scores
const scoreList = document.getElementById('scores');
const sorted = Object.entries(result.scores || {}).sort((a,b) => b[1]-a[1]);
sorted.forEach(([name, score], i) => {
  const li = document.createElement('li');
  li.className = 'score-item';
  li.innerHTML = '<span class="score-rank">' + (medals[i]||i+1) + '</span>' +
    '<span class="score-name">' + name + '</span>' +
    '<span class="score-pts">' + score + '点</span>';
  scoreList.appendChild(li);
});

// Chain
const history = result.history || [];
const chain = history.map(h => h.word).join(' → ');
document.getElementById('chain').textContent = chain || '(なし)';

// History list
const hList = document.getElementById('history');
history.forEach((h, i) => {
  const li = document.createElement('li');
  li.innerHTML = '<span class="h-num">' + (i+1) + '.</span>' +
    '<span class="h-word">' + h.word + '</span>' +
    '<span class="h-player">' + h.player + '</span>';
  hList.appendChild(li);
});
</script>
</body>
</html>`
