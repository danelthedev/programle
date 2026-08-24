package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

type Lang struct {
	Name     string   `json:"name"`
	Snippets []string `json:"snippets"`
}

var langs []Lang
var langsJSON string

func main() {
	b, err := os.ReadFile("data.json")
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(b, &langs); err != nil {
		log.Fatal(err)
	}
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.Encode(langs)
	langsJSON = strings.TrimSpace(buf.String())

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/", homeHandler)

	log.Println("http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, l := range langs {
		if q == "" || strings.Contains(strings.ToLower(l.Name), q) {
			// ponytail: no escape needed, names are simple
			w.Write([]byte(`<button class="btn btn-ghost btn-sm w-full justify-start font-mono" onclick="pick('` + l.Name + `')">` + l.Name + `</button>`))
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Programle</title>
<link href="https://cdn.jsdelivr.net/npm/daisyui@5" rel="stylesheet" type="text/css" />
<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>
<script src="/static/js/htmx.min.js"></script>
<style>.font-mono{font-family:ui-monospace,monospace}@keyframes blink{0%,49%{opacity:1}50%,100%{opacity:0}}.blink{animation:blink 1s step-end infinite}</style>
</head>
<body class="min-h-screen bg-base-300 flex flex-col items-center p-4 gap-5">
<header class="pt-6 text-center">
  <h1 class="font-mono text-5xl md:text-6xl font-black tracking-tighter"><span class="text-primary">></span> PROGRAMLE<span class="text-primary blink">_</span></h1>
</header>

<button id="modeToggle" class="btn btn-sm font-mono absolute top-4 right-4 z-50">DAILY</button>

<div class="card bg-base-200 w-full max-w-2xl shadow-xl border border-base-100">
  <div class="card-body p-4">
    <pre class="overflow-x-auto"><code id="snippet" class="font-mono text-sm whitespace-pre-wrap break-words"></code></pre>
  </div>
</div>

<div id="guesses" class="w-full max-w-2xl grid gap-2">
  <div class="h-10 rounded-box border border-base-100 bg-base-200 border-dashed opacity-60"></div>
  <div class="h-10 rounded-box border border-base-100 bg-base-200 border-dashed opacity-60"></div>
  <div class="h-10 rounded-box border border-base-100 bg-base-200 border-dashed opacity-60"></div>
  <div class="h-10 rounded-box border border-base-100 bg-base-200 border-dashed opacity-60"></div>
  <div class="h-10 rounded-box border border-base-100 bg-base-200 border-dashed opacity-60"></div>
  <div class="h-10 rounded-box border border-base-100 bg-base-200 border-dashed opacity-60"></div>
</div>

<div class="w-full max-w-2xl">
  <div class="relative">
    <input id="guessInput" type="text" placeholder="Type or click to see languages..." autocomplete="off"
      class="input input-bordered w-full font-mono"
      name="q" hx-get="/search" hx-trigger="keyup changed, focus, click" hx-target="#options">
    <div id="options" class="absolute z-10 bg-base-200 w-full mt-1 rounded-box shadow-lg max-h-60 overflow-y-auto hidden border border-base-100 p-1"></div>
  </div>
  <button id="submitBtn" class="btn btn-primary w-full mt-2 font-mono">GUESS</button>
  <p id="message" class="text-center font-mono text-sm mt-2 min-h-5"></p>
  <button id="restartBtn" class="btn btn-ghost w-full mt-1 font-mono hidden">Play again</button>
</div>

<script>
const LANGS = `+langsJSON+`;
let mode = localStorage.getItem('mode') || 'daily';
let target = null;
let guesses = [];
let level = 0;

const snippetEl = document.getElementById('snippet');
const guessesEl = document.getElementById('guesses');
const inputEl = document.getElementById('guessInput');
const optionsEl = document.getElementById('options');
const messageEl = document.getElementById('message');
const toggleEl = document.getElementById('modeToggle');
const restartBtn = document.getElementById('restartBtn');
document.body.addEventListener('htmx:afterSwap', e=>{ if(e.detail.target && e.detail.target.id==='options') e.detail.target.classList.remove('hidden'); });

function dailyIndex(){ return Math.floor(Date.now()/86400000) % LANGS.length; }
function pickTarget(){
  if(mode==='daily') return LANGS[dailyIndex()];
  return LANGS[Math.floor(Math.random()*LANGS.length)];
}
function renderSnippet(){
  snippetEl.textContent = target.snippets[Math.min(level, target.snippets.length-1)];
}
function renderGuesses(){
  guessesEl.innerHTML='';
  for(let i=0;i<6;i++){
    const g = guesses[i];
    const div = document.createElement('div');
    div.className='h-10 rounded-box border flex items-center px-3 font-mono text-sm ' + (g ? (g.correct?'bg-success text-success-content border-success':'bg-error text-error-content border-error') : 'border-base-100 bg-base-200 border-dashed opacity-60');
    div.textContent = g ? g.name : '';
    guessesEl.appendChild(div);
  }
}
function renderOptions(q){ optionsEl.innerHTML=''; const f=(q||'').toLowerCase(); const filtered=LANGS.filter(l=>!f||l.name.toLowerCase().includes(f)); filtered.forEach(l=>{ const b=document.createElement('button'); b.className='btn btn-ghost btn-sm w-full justify-start font-mono'; b.textContent=l.name; b.onclick=()=>pick(l.name); optionsEl.appendChild(b); }); if(filtered.length) optionsEl.classList.remove('hidden'); else optionsEl.classList.add('hidden'); }
function setMessage(t, cls=''){ messageEl.textContent=t; messageEl.className='text-center font-mono text-sm mt-2 min-h-5 '+cls; }
function resetGame(){
  target = pickTarget();
  guesses = [];
  level = 0;
  inputEl.value=''; inputEl.disabled=false; document.getElementById('submitBtn').disabled=false;
  restartBtn.classList.add('hidden'); setMessage('');
  renderSnippet(); renderGuesses();
  // refresh htmx options hidden
  optionsEl.classList.add('hidden'); optionsEl.innerHTML='';
}
function pick(name){ inputEl.value=name; optionsEl.classList.add('hidden'); }
window.pick = pick;

function submitGuess(){
  const raw = inputEl.value.trim();
  if(!raw) return;
  const found = LANGS.find(l=>l.name.toLowerCase()===raw.toLowerCase());
  if(!found){ setMessage('Not in list','text-error'); return; }
  if(guesses.some(x=>x.name.toLowerCase()===found.name.toLowerCase())){ setMessage('Already guessed','text-warning'); return; }
  const correct = found.name===target.name;
  guesses.push({name:found.name, correct});
  if(!correct) level = Math.min(level+1, target.snippets.length-1);
  renderGuesses(); renderSnippet();
  inputEl.value=''; optionsEl.classList.add('hidden'); setMessage('');
  if(correct){
    setMessage('Correct! '+target.name+' — '+(guesses.length)+'/6','text-success');
    inputEl.disabled=true; document.getElementById('submitBtn').disabled=true;
    restartBtn.classList.remove('hidden');
    if(mode==='daily') localStorage.setItem('daily-'+dailyIndex(), JSON.stringify(guesses));
  } else if(guesses.length>=6){
    setMessage('Out of guesses! It was '+target.name,'text-error');
    inputEl.disabled=true; document.getElementById('submitBtn').disabled=true;
    restartBtn.classList.remove('hidden');
  }
  // daily persist
  if(mode==='daily') localStorage.setItem('guesses-daily-'+dailyIndex(), JSON.stringify({guesses, level}));
}

// events
document.getElementById('submitBtn').addEventListener('click', submitGuess);
inputEl.addEventListener('keydown', e=>{ if(e.key==='Enter'){ e.preventDefault(); submitGuess(); } if(e.key==='Escape') optionsEl.classList.add('hidden'); });
inputEl.addEventListener('input', e=> renderOptions(e.target.value));
inputEl.addEventListener('focus', e=> renderOptions(e.target.value));
inputEl.addEventListener('click', e=> renderOptions(e.target.value));
document.addEventListener('click', e=>{ if(!e.target.closest('#guessInput') && !e.target.closest('#options')) optionsEl.classList.add('hidden'); });
restartBtn.addEventListener('click', ()=>{ if(mode==='unlimited') resetGame(); else { mode='unlimited'; localStorage.setItem('mode','unlimited'); updateToggleUI(); resetGame(); } });

// toggle
function updateToggleUI(){ toggleEl.textContent = mode==='daily' ? 'DAILY' : 'UNLIMITED'; }
toggleEl.addEventListener('click', ()=>{
  mode = mode==='daily' ? 'unlimited' : 'daily';
  localStorage.setItem('mode', mode);
  updateToggleUI();
  if(mode==='daily'){
    const saved = localStorage.getItem('guesses-daily-'+dailyIndex());
    if(saved){
      try{
        const s=JSON.parse(saved);
        guesses=s.guesses||[];
        level=s.level||guesses.length;
        target=pickTarget();
        renderSnippet();
        renderGuesses();
        if(guesses.some(g=>g.correct)||guesses.length>=6){
          inputEl.disabled=true;
          document.getElementById('submitBtn').disabled=true;
          setMessage(guesses.some(g=>g.correct)?'Already solved!':'Out of guesses! It was '+target.name, guesses.some(g=>g.correct)?'text-success':'text-error');
          restartBtn.classList.remove('hidden');
        }
        return;
      }catch(e){}
    }
  }
  resetGame();
});
updateToggleUI();

// init
target = pickTarget();
// restore daily if exists
if(mode==='daily'){
  const saved = localStorage.getItem('guesses-daily-'+dailyIndex());
  if(saved){ try{ const s=JSON.parse(saved); guesses=s.guesses||[]; level=s.level||0; if(guesses.length) { renderSnippet(); renderGuesses(); if(guesses.some(g=>g.correct)||guesses.length>=6){ inputEl.disabled=true; document.getElementById('submitBtn').disabled=true; } } }catch(e){} }
}
if(!guesses.length) resetGame();
else { renderSnippet(); renderGuesses(); }
</script>
</body>
</html>`))
}
