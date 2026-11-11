/* Standalone local study. React integration will own an equivalent scoped lifecycle. */
(() => {
  const root = document.querySelector('.about-story');
  const reduce = document.querySelector('#reduce-motion');
  const status = document.querySelector('#motion-status');
  const preference = matchMedia('(prefers-reduced-motion: reduce)');
  const mobile = matchMedia('(max-width: 620px)');
  const short = matchMedia('(max-height: 620px)');
  const finePointer = matchMedia('(hover: hover) and (pointer: fine)');
  const { gsap, ScrollTrigger } = window;
  let context, entrance;
  let cleanup = [];
  let disposed = false;
  if (!gsap || !ScrollTrigger) { status.textContent = 'Static preview · animation files unavailable'; return; }
  gsap.registerPlugin(ScrollTrigger);
  function rebuild(enter = false) {
    if (disposed) return;
    for (const stop of cleanup) stop();
    cleanup = [];
    context?.revert();
    root.classList.remove('is-motion');
    entrance = null;
    const enlarged = parseFloat(getComputedStyle(document.documentElement).fontSize) > 20;
    if (preference.matches || reduce.checked || short.matches || enlarged) {
      status.textContent = preference.matches ? 'Static · system reduced motion' : 'Static · reduced motion or reading layout';
      return;
    }
    try {
      context = gsap.context(() => {
        root.classList.add('is-motion');
        const slots = gsap.utils.toArray('.petal-slot');
        const scatter = [
          [20,10],[56,8],[88,20],[14,44],[48,38],[84,48],
          [4,78],[39,76],[75,83],[100,72],[55,104],[12,109],
        ];
        const scene = gsap.timeline({defaults:{ease:'power1.inOut'},scrollTrigger:{trigger:root,start:'top top+=72',end:'bottom bottom',scrub:.65,invalidateOnRefresh:true}});
        scene.to('.warm-field',{right:'52%',top:'0%',width:'48%',height:'100%',duration:1},0)
          .to('.orbit-layout',{left:mobile.matches?'24%':'37%',top:mobile.matches?'6%':'-10%',width:mobile.matches?'115vmin':'105vmin',duration:1},0)
          .to('.orbit-rig',{rotation:72,duration:1},0)
          .to(slots,{left:i=>scatter[i][0]+'%',top:i=>scatter[i][1]+'%',width:mobile.matches?'27%':'28%',duration:1},.1)
          .to('.warm-field',{right:'-30%',top:'15%',width:'55%',height:'78%',duration:1},1.05)
          .to('.orbit-layout',{left:mobile.matches?'-20%':'-9%',top:mobile.matches?'22%':'4%',width:mobile.matches?'125vmin':'95vmin',duration:1},1.05)
          .to('.orbit-rig',{rotation:148,duration:1},1.05)
          .to(slots,{left:i=>16+(i%4)*24+'%',top:i=>20+Math.floor(i/4)*28+'%',width:'34%',duration:1},1.1)
          .to('.warm-field',{right:'0%',top:'0%',width:mobile.matches?'32%':'34%',height:'40%',duration:1},2.1)
          .to('.orbit-layout',{left:mobile.matches?'45%':'64%',top:mobile.matches?'7%':'-12%',width:mobile.matches?'115vmin':'88vmin',duration:1},2.1)
          .to('.orbit-rig',{rotation:240,duration:1},2.1)
          .to(slots,{left:i=>50+Math.cos(i*Math.PI/6)*30+'%',top:i=>50+Math.sin(i*Math.PI/6)*30+'%',width:'34%',duration:1},2.1)
          .to('.orbit-rig',{rotation:260,duration:.4},3.1);
        if (enter && !document.hidden && scrollY < 100) {
          entrance = gsap.timeline();
          entrance.from('.title-line > span',{yPercent:24,opacity:0,duration:.85,stagger:.12,ease:'power3.out'},.08)
            .from('.chapter-label',{opacity:0,duration:.6},.1);
        }
        root.querySelectorAll('.interest-row').forEach(row => {
          const reveal = gsap.from(row,{y:24,opacity:0,duration:.65,ease:'power2.out',immediateRender:false,scrollTrigger:{trigger:row,start:'top 92%',once:true}});
          const show = () => { reveal.progress(1).pause();reveal.scrollTrigger?.kill(); };
          row.addEventListener('focusin',show);cleanup.push(()=>row.removeEventListener('focusin',show));
        });
        const pointer = root.querySelector('.pointer-layer');
        const x = gsap.quickTo(pointer,'x',{duration:.7,ease:'power3.out'});
        const y = gsap.quickTo(pointer,'y',{duration:.7,ease:'power3.out'});
        const leave = () => { x(0);y(0); };
        const move = event => {
          if (!finePointer.matches || event.pointerType !== 'mouse') return;
          x(gsap.utils.clamp(-14,14,(event.clientX/innerWidth-.5)*28));
          y(gsap.utils.clamp(-10,10,(event.clientY/innerHeight-.5)*20));
        };
        root.addEventListener('pointermove',move,{passive:true});root.addEventListener('pointerleave',leave);window.addEventListener('blur',leave);
        cleanup.push(()=>{root.removeEventListener('pointermove',move);root.removeEventListener('pointerleave',leave);window.removeEventListener('blur',leave);});
      },root);
      ScrollTrigger.refresh();
      status.textContent = 'Motion on · scroll to explore';
    } catch {
      for (const stop of cleanup) stop(); cleanup=[];context?.revert();root.classList.remove('is-motion');
      status.textContent='Static preview · animation unavailable';
    }
  }
  const rebuildSettings = () => rebuild(false);
  const visibility = () => {
    document.documentElement.classList.toggle('motion-paused',document.hidden);
    if (document.hidden) entrance?.pause(); else entrance?.resume();
  };
  let fontSize = getComputedStyle(document.documentElement).fontSize;
  const textObserver = new ResizeObserver(() => {
    const next = getComputedStyle(document.documentElement).fontSize;
    if (next !== fontSize) {fontSize=next;rebuild(false);}
  });
  textObserver.observe(root.querySelector('.hero-copy'));
  reduce.addEventListener('change',rebuildSettings);preference.addEventListener('change',rebuildSettings);mobile.addEventListener('change',rebuildSettings);short.addEventListener('change',rebuildSettings);document.addEventListener('visibilitychange',visibility);
  const restore = () => {
    if (!location.hash) return;
    let target;try{target=document.getElementById(decodeURIComponent(location.hash.slice(1)));}catch{return;}
    if (target) {target.scrollIntoView({block:'start'});ScrollTrigger.update();}
  };
  window.addEventListener('load',restore);
  const destroy = () => {
    disposed=true;for(const stop of cleanup)stop();cleanup=[];context?.revert();root.classList.remove('is-motion');textObserver.disconnect();
    reduce.removeEventListener('change',rebuildSettings);preference.removeEventListener('change',rebuildSettings);mobile.removeEventListener('change',rebuildSettings);short.removeEventListener('change',rebuildSettings);document.removeEventListener('visibilitychange',visibility);window.removeEventListener('load',restore);
  };
  window.addEventListener('pagehide',event=>{if(!event.persisted)destroy();});
  rebuild(true);visibility();
})();
