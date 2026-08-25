let mode = localStorage.getItem('mode') || 'daily';
let target = null;
let guesses = [];
let level = 0;
let sIdx = 0;
const snippetEl = document.getElementById('snippet');
const guessesEl = document.getElementById('guesses');
const inputEl = document.getElementById('guessInput');
const optionsEl = document.getElementById('options');
const messageEl = document.getElementById('message');
const toggleEl = document.getElementById('modeToggle');
const restartBtn = document.getElementById('restartBtn');
const popularityEl = document.getElementById('popularity');
const popularityBtn = document.getElementById('popularityBtn');
const popLangEl = document.getElementById('popLang');
const popDatasetEl = document.getElementById('popDataset');
const popDescEl = document.getElementById('popDesc');
const popLineEl = document.getElementById('popLine');
const popPoolEl = document.getElementById('popPool');
const popOrderSection = document.getElementById('popOrderSection');
const popRankSection = document.getElementById('popRankSection');
const popRankInput = document.getElementById('popRankInput');
const popRankSlider = document.getElementById('popRankSlider');
const popRankVal = document.getElementById('popRankVal');
const popOrderResult = document.getElementById('popOrderResult');
const popRankResult = document.getElementById('popRankResult');
let popDatasets = [];
let popIdx = 0;
let popPool = [];
let popSelection = [];
document.body.addEventListener('htmx:afterSwap', e=>{ if(e.detail.target && e.detail.target.id==='options') e.detail.target.classList.remove('hidden'); });
function dailyIndex(){ return Math.floor(Date.now()/86400000) % LANGS.length; }
function pickTarget(){
  if(mode==='daily') return LANGS[dailyIndex()];
  return LANGS[Math.floor(Math.random()*LANGS.length)];
}
function renderSnippet(){
  const arr = target.snippets||[];
  const unlocked = Math.min(arr.length, level+1);
  const i = Math.min(sIdx, unlocked-1);
  if(sIdx >= unlocked) sIdx = unlocked-1;
  snippetEl.textContent = arr[Math.max(0,i)]||"";
  const idxEl=document.getElementById('snippetIdx');
  if(idxEl) idxEl.textContent = (arr.length? (Math.min(sIdx,unlocked-1)+1)+"/"+unlocked+" (of "+arr.length+")" : "");
  document.getElementById('prevBtn').disabled = sIdx<=0;
  document.getElementById('nextBtn').disabled = sIdx>=unlocked-1;
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
function positionOptions(){
  if(optionsEl.classList.contains('hidden')) return;
  const rect=inputEl.getBoundingClientRect();
  const spaceBelow=window.innerHeight - rect.bottom;
  const spaceAbove=rect.top;
  const est=Math.min(240, optionsEl.children.length*36 + 8);
  const needed=optionsEl.scrollHeight ? Math.min(240, optionsEl.scrollHeight) : est;
  const openUp= needed > spaceBelow && spaceAbove > spaceBelow;
  optionsEl.classList.toggle('top-full', !openUp);
  optionsEl.classList.toggle('bottom-full', openUp);
  optionsEl.classList.toggle('mt-1', !openUp);
  optionsEl.classList.toggle('mb-1', openUp);
}
function renderOptions(q){ optionsEl.innerHTML=''; const f=(q||'').toLowerCase(); const guessed=new Set(guesses.map(g=>g.name.toLowerCase())); const filtered=LANGS.filter(l=> (!f||l.name.toLowerCase().includes(f)) && !guessed.has(l.name.toLowerCase())); filtered.forEach(l=>{ const b=document.createElement('button'); b.className='btn btn-ghost btn-sm w-full justify-start font-mono'; b.textContent=l.name; b.onclick=()=>pick(l.name); optionsEl.appendChild(b); }); if(filtered.length){ optionsEl.classList.remove('hidden'); requestAnimationFrame(positionOptions); } else optionsEl.classList.add('hidden'); }
window.addEventListener('resize', positionOptions);
window.addEventListener('scroll', positionOptions, true);
function setMessage(t, cls=''){ messageEl.textContent=t; messageEl.className='text-center font-mono text-sm mt-2 min-h-5 '+cls; }
document.getElementById('prevBtn').onclick=()=>{ if(sIdx>0){sIdx--;renderSnippet();}};
document.getElementById('nextBtn').onclick=()=>{ const unlocked=Math.min(target.snippets.length, level+1); if(sIdx<unlocked-1){sIdx++;renderSnippet();}};
function resetGame(){
  target = pickTarget();
  guesses = [];
  level = 0; sIdx=0;
  inputEl.value=''; inputEl.disabled=false; document.getElementById('submitBtn').disabled=false;
  restartBtn.classList.add('hidden'); setMessage('');
  if(popularityBtn) popularityBtn.classList.add('hidden');
  if(popularityEl) popularityEl.classList.add('hidden');
  document.getElementById('mainGame')?.classList.remove('hidden');
  popDatasets=[]; popIdx=0; popPool=[]; popSelection=[];
  renderSnippet(); renderGuesses();
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
  if(!correct){ level = Math.min(level+1, target.snippets.length-1); sIdx = level; }
  renderGuesses(); renderSnippet();
  inputEl.value=''; optionsEl.classList.add('hidden'); setMessage('');
  if(correct){
    setMessage('Correct! '+target.name+' — '+(guesses.length)+'/6','text-success');
    inputEl.disabled=true; document.getElementById('submitBtn').disabled=true;
    if((target.tiobe_rank!=null || target.github_rank!=null) && popularityBtn){
      popularityBtn.classList.remove('hidden');
      restartBtn.classList.add('hidden');
    } else {
      restartBtn.classList.remove('hidden');
    }
  } else if(guesses.length>=6){
    setMessage('Out of guesses! It was '+target.name,'text-error');
    inputEl.disabled=true; document.getElementById('submitBtn').disabled=true;
    if((target.tiobe_rank!=null || target.github_rank!=null) && popularityBtn){
      popularityBtn.classList.remove('hidden');
      restartBtn.classList.add('hidden');
    } else {
      restartBtn.classList.remove('hidden');
    }
  }
  if(mode==='daily') localStorage.setItem('guesses-daily-'+dailyIndex(), JSON.stringify({guesses, level}));
}
document.getElementById('submitBtn').addEventListener('click', submitGuess);
inputEl.addEventListener('keydown', e=>{ if(e.key==='Enter'){ e.preventDefault(); submitGuess(); } if(e.key==='Escape') optionsEl.classList.add('hidden'); });
inputEl.addEventListener('input', e=> renderOptions(e.target.value));
inputEl.addEventListener('click', e=> renderOptions(e.target.value));
document.addEventListener('click', e=>{ if(!e.target.closest('#guessInput') && !e.target.closest('#options')) optionsEl.classList.add('hidden'); });
restartBtn.addEventListener('click', ()=>{ if(mode==='unlimited') resetGame(); else { mode='unlimited'; localStorage.setItem('mode','unlimited'); updateToggleUI(); resetGame(); } });
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
          if((target.tiobe_rank!=null || target.github_rank!=null) && popularityBtn){
            popularityBtn.classList.remove('hidden');
            restartBtn.classList.add('hidden');
          } else {
            restartBtn.classList.remove('hidden');
          }
        }
        return;
      }catch(e){}
    }
  }
  resetGame();
});
updateToggleUI();
target = pickTarget();
if(mode==='daily'){
  const saved = localStorage.getItem('guesses-daily-'+dailyIndex());
  if(saved){ try{ const s=JSON.parse(saved); guesses=s.guesses||[]; level=s.level||0; if(guesses.length) { renderSnippet(); renderGuesses(); if(guesses.some(g=>g.correct)||guesses.length>=6){ inputEl.disabled=true; document.getElementById('submitBtn').disabled=true; } } }catch(e){} }
}
if(!guesses.length) resetGame();
else { renderSnippet(); renderGuesses(); }
optionsEl.classList.add('hidden'); optionsEl.innerHTML='';

// popularity minigame: current language +4, order then rank per dataset, no points — just tracked
function getPopDatasets(){ const ds=[]; if(target && target.tiobe_rank!=null) ds.push('tiobe'); if(target && target.github_rank!=null) ds.push('github'); return ds; }
function shuffle(a){ for(let i=a.length-1;i>0;i--){ const j=Math.floor(Math.random()*(i+1)); [a[i],a[j]]=[a[j],a[i]];} return a; }
function pickPopPool(){
  const ds=popDatasets[popIdx]; const key=ds==='tiobe'?'tiobe_rank':'github_rank';
  const tr=target[key];
  let cands=LANGS.filter(l=> l[key]!=null && l.name!==target.name);
  cands.sort((a,b)=> Math.abs(a[key]-tr)-Math.abs(b[key]-tr));
  cands=cands.slice(0,12);
  shuffle(cands);
  const chosen=cands.slice(0,4);
  popPool=shuffle([...chosen, target]);
  popSelection=[];
}
function renderPopLine(){
  if(!popLineEl) return;
  popLineEl.innerHTML='';
  for(let i=0;i<5;i++){
    const d=document.createElement('div');
    d.className='h-12 rounded-box border flex items-center justify-center px-2 font-mono text-sm font-bold truncate overflow-hidden whitespace-nowrap cursor-pointer '+(popSelection[i]?'bg-base-100 border-base-300':'border-dashed opacity-60 border-base-100');
    d.textContent=popSelection[i]|| (i+1);
    d.onclick=()=>{ if(popSelection[i]){ popSelection.splice(i,1); renderPop(); }};
    popLineEl.appendChild(d);
  }
}
function renderPopPool(){
  if(!popPoolEl) return;
  popPoolEl.innerHTML='';
  popPool.forEach(l=>{
    if(popSelection.includes(l.name)) return;
    const b=document.createElement('button');
    b.className='btn btn-ghost font-mono text-sm';
    b.textContent=l.name;
    b.onclick=()=>{ if(popSelection.length<5){ popSelection.push(l.name); renderPop(); }};
    popPoolEl.appendChild(b);
  });
}
function renderPop(){ renderPopLine(); renderPopPool(); const sub=document.getElementById('popOrderSubmit'); if(sub) sub.disabled=popSelection.length!==5; }
function startPopDataset(){
  const ds=popDatasets[popIdx];
  if(!ds) return;
  const isTiobe=ds==='tiobe';
  if(popDatasetEl) popDatasetEl.textContent=isTiobe?'TIOBE':'GitHub';
  if(popLangEl) popLangEl.textContent=target.name;
  document.querySelectorAll('.popLangRank').forEach(e=> e.textContent=target.name);
  document.querySelectorAll('.popDatasetRank').forEach(e=> e.textContent=isTiobe?'TIOBE':'GitHub');
  if(popDescEl) popDescEl.textContent=isTiobe?'Order most popular → least popular (TIOBE ranking)':'Order most popular → least popular (GITHUB active repositories)';
  pickPopPool();
  if(popOrderSection) popOrderSection.classList.remove('hidden');
  if(popRankSection) popRankSection.classList.add('hidden');
  if(popOrderResult) { popOrderResult.classList.add('hidden'); popOrderResult.textContent=''; }
  if(popRankResult){ popRankResult.classList.add('hidden'); popRankResult.textContent=''; }
  const ranksEl=document.getElementById('popOrderRanks'); if(ranksEl){ ranksEl.innerHTML=''; ranksEl.classList.add('hidden'); }
  if(popPoolEl) popPoolEl.classList.remove('hidden');
  const sub=document.getElementById('popOrderSubmit'); if(sub){ sub.disabled=false; sub.classList.remove('opacity-50'); }
  const nextBtn=document.getElementById('popNextDataset'); if(nextBtn) nextBtn.classList.add('hidden');
  const closeBtn=document.getElementById('popClose'); if(closeBtn) closeBtn.classList.add('hidden');
  const key=isTiobe?'tiobe_rank':'github_rank'; const max=isTiobe?50:275;
  if(popRankSlider){ popRankSlider.max=max; popRankSlider.value=String(Math.min(25,max)); popRankSlider.disabled=false; popRankSlider.className='range range-sm flex-1 range-primary'; }
  if(popRankInput){ popRankInput.max=max; popRankInput.value=String(Math.min(25,max)); popRankInput.disabled=false; popRankInput.className='input input-bordered input-sm w-24 font-mono'; }
  const rankSub=document.getElementById('popRankSubmit'); if(rankSub){ rankSub.disabled=false; rankSub.className='btn btn-primary btn-sm font-mono'; }
  if(popRankVal) popRankVal.textContent=popRankSlider?popRankSlider.value:'';
  renderPop();
}
function startPopularity(){
  popDatasets=getPopDatasets();
  if(!popDatasets.length){ alert('No popularity data for '+target.name); return; }
  popIdx=0;
  document.getElementById('mainGame')?.classList.add('hidden');
  if(popularityBtn) popularityBtn.classList.add('hidden');
  if(popularityEl) { popularityEl.classList.remove('hidden'); popularityEl.scrollIntoView({behavior:'smooth'}); }
  startPopDataset();
}
function submitPopOrder(){
  const ds=popDatasets[popIdx]; const key=ds==='tiobe'?'tiobe_rank':'github_rank';
  const poolMap=new Map(popPool.map(l=>[l.name,l]));
  const actual=[...popPool].sort((a,b)=>a[key]-b[key]).map(l=>l.name);
  const slots=popLineEl?.children || [];
  for(let i=0;i<5;i++){
    const name=popSelection[i]; const ok=name===actual[i];
    const el=slots[i]; if(!el) continue;
    el.className='h-12 rounded-box border flex items-center justify-center px-2 font-mono text-sm font-bold truncate overflow-hidden whitespace-nowrap '+(ok?'bg-success text-success-content border-success':'bg-error text-error-content border-error');
    el.onclick=null; el.style.pointerEvents='none';
  }
  // keep #ranks hidden until rank is also guessed
  const ranksEl=document.getElementById('popOrderRanks'); if(ranksEl){ ranksEl.classList.add('hidden'); }
  if(popPoolEl) popPoolEl.classList.add('hidden');
  const sub=document.getElementById('popOrderSubmit'); if(sub) { sub.disabled=true; sub.classList.add('opacity-50'); }
  if(popOrderResult){ popOrderResult.textContent='Correct order: '+actual.join(' → '); popOrderResult.classList.remove('hidden'); }
  if(popRankSection) popRankSection.classList.remove('hidden');
}
function submitPopRank(){
  const ds=popDatasets[popIdx]; const key=ds==='tiobe'?'tiobe_rank':'github_rank';
  const actual=target[key];
  let guess=parseInt(popRankInput?.value || popRankSlider?.value,10);
  if(isNaN(guess)) { if(popRankResult){ popRankResult.textContent='Enter a number'; popRankResult.className='font-mono text-xs text-error text-center'; popRankResult.classList.remove('hidden'); } return; }
  const maxRank=ds==='tiobe'?50:275; const tolerance=Math.ceil(maxRank*0.05); const ok= Math.abs(guess-actual) <= tolerance;
  // color rank input/slider/submit
  if(popRankInput) popRankInput.className='input input-bordered input-sm w-24 font-mono '+(ok?'border-success text-success':'border-error text-error');
  if(popRankSlider) popRankSlider.className='range range-sm flex-1 '+(ok?'range-success':'range-error');
  const rankSub=document.getElementById('popRankSubmit'); if(rankSub){ rankSub.className='btn btn-sm font-mono '+(ok?'btn-success':'btn-error'); rankSub.disabled=true; }
  if(popRankInput) popRankInput.disabled=true;
  if(popRankSlider) popRankSlider.disabled=true;
  // show #ranks for order phase now that rank also guessed
  const poolMap=new Map(popPool.map(l=>[l.name,l]));
  const actualOrder=[...popPool].sort((a,b)=>a[key]-b[key]).map(l=>l.name);
  const ranksEl=document.getElementById('popOrderRanks');
  if(ranksEl){
    ranksEl.innerHTML='';
    for(let i=0;i<5;i++){
      const name=popSelection[i]; const l=poolMap.get(name); const isOk=name===actualOrder[i];
      const d=document.createElement('div');
      d.className='font-mono text-sm font-bold '+(isOk?'text-success':'text-error');
      d.textContent=l? '#'+l[key] : '';
      ranksEl.appendChild(d);
    }
    ranksEl.classList.remove('hidden');
  }
  // hide #number below rank button
  if(popRankResult) popRankResult.classList.add('hidden');
  const nextBtn=document.getElementById('popNextDataset'); const closeBtn=document.getElementById('popClose');
  if(popIdx < popDatasets.length-1){ if(nextBtn) nextBtn.classList.remove('hidden'); } else { if(closeBtn) closeBtn.classList.remove('hidden'); }
}
if(popularityBtn) popularityBtn.addEventListener('click', startPopularity);
document.getElementById('popOrderSubmit')?.addEventListener('click', submitPopOrder);
document.getElementById('popRankSubmit')?.addEventListener('click', submitPopRank);
document.getElementById('popNextDataset')?.addEventListener('click', ()=>{ popIdx++; startPopDataset(); });
document.getElementById('popClose')?.addEventListener('click', ()=>{ if(popularityEl) popularityEl.classList.add('hidden'); document.getElementById('mainGame')?.classList.remove('hidden'); restartBtn.classList.remove('hidden'); });
popRankSlider?.addEventListener('input', e=>{ if(popRankVal) popRankVal.textContent=e.target.value; if(popRankInput) popRankInput.value=e.target.value; });
popRankInput?.addEventListener('input', e=>{ if(popRankSlider) popRankSlider.value=e.target.value; if(popRankVal) popRankVal.textContent=e.target.value; });
