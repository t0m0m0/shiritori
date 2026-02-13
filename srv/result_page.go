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
<link href="https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@300;400;500;700;900&display=swap" rel="stylesheet">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --primary:#6366f1;--primary-light:#818cf8;--primary-dark:#4f46e5;
  --accent:#f59e0b;--accent-light:#fbbf24;
  --danger:#ef4444;--success:#22c55e;
  --bg:#f8f7ff;--surface:#fff;--surface2:#f1f0fb;
  --text:#1e1b4b;--text2:#6b7280;
  --radius:12px;--shadow:0 4px 24px rgba(99,102,241,.10);
  --border:#e5e7eb;
}
body{
  font-family:'Noto Sans JP',system-ui,sans-serif;
  background:var(--bg);color:var(--text);
  min-height:100dvh;line-height:1.6;
}
.header{
  text-align:center;padding:2rem 1rem 1rem;
  background:linear-gradient(135deg,#6366f1 0%%,#818cf8 50%%,#a78bfa 100%%);
  color:#fff;
}
.header h1{font-size:2rem;font-weight:900}
.container{max-width:600px;margin:0 auto;padding:1.5rem 1rem}
.card{
  background:var(--surface);border-radius:var(--radius);
  padding:1.5rem;box-shadow:var(--shadow);margin-bottom:1rem;
}
.card h2{
  font-size:1.1rem;margin-bottom:1rem;color:var(--primary-dark);
  border-left:3px solid var(--primary);padding-left:.5rem;
}
.reason{text-align:center;font-size:1.1rem;font-weight:700;color:var(--danger);margin-bottom:.5rem}
.genre-tag{
  text-align:center;font-size:.85rem;color:var(--primary);
  margin-bottom:1rem;
}
.scores{list-style:none}
.score-item{
  display:flex;justify-content:space-between;align-items:center;
  padding:.6rem 1rem;border-radius:8px;margin-bottom:.4rem;
  background:var(--surface2);
}
.score-item:first-child{background:linear-gradient(90deg,#fef3c7,#fde68a);font-weight:700}
.score-rank{width:2rem;text-align:center;font-weight:700}
.score-name{flex:1;text-align:left;margin-left:.5rem}
.score-pts{font-weight:700;color:var(--primary)}
.chain-summary{
  font-size:.85rem;color:var(--text2);padding:.5rem .75rem;
  background:var(--surface2);border-radius:8px;margin-bottom:.75rem;
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
.cta{
  text-align:center;margin-top:1.5rem;
}
.btn{
  display:inline-block;padding:.75rem 2rem;border-radius:999px;
  font-weight:700;font-size:1rem;text-decoration:none;
  background:var(--primary);color:#fff;
  transition:background .2s;
}
.btn:hover{background:var(--primary-dark)}
.footer{text-align:center;padding:2rem;color:var(--text2);font-size:.8rem}
</style>
</head>
<body>
<div class="header">
  <h1>🎌 しりとり</h1>
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
    <a class="btn" href="/">🎮 しりとりで遊ぶ</a>
  </div>
</div>
<div class="footer">しりとり - マルチプレイヤー</div>
<script>
const result = %s;
const medals = ['🥇','🥈','🥉'];

// Reason
let reason = result.reason || '';
if (result.winner) reason = '🏆 ' + result.winner + 'さんの勝利！';
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
