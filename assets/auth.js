(()=>{
  const gate=document.getElementById('authGate');if(!gate)return;
  const form=document.getElementById('authForm'),username=document.getElementById('authUsername'),password=document.getElementById('authPassword');
  const submit=document.getElementById('authSubmit'),status=document.getElementById('authStatus'),logout=document.getElementById('authLogout');
  const setStatus=(message,isError=false)=>{status.textContent=message;status.classList.toggle('error',isError)};
  const showGate=message=>{gate.hidden=false;document.body.classList.add('auth-locked');setStatus(message,Boolean(message&&message!=='请输入客服账号和密码'))};
  const hideGate=name=>{
    gate.hidden=true;document.body.classList.remove('auth-locked');
    const account=document.getElementById('authAccountName');if(account)account.textContent=name||localStorage.getItem('auth_username')||'客服账号';
    document.querySelectorAll('iframe[data-auth-src]').forEach(frame=>{if(!frame.src)frame.src=frame.dataset.authSrc});
    document.dispatchEvent(new CustomEvent('auth:ready'));
  };
  const clearAuth=()=>{localStorage.removeItem('token');localStorage.removeItem('auth_username')};
  logout.addEventListener('click',()=>{clearAuth();location.reload()});
  form.addEventListener('submit',async event=>{
    event.preventDefault();const name=username.value.trim(),pass=password.value;
    if(!name||!pass){setStatus('请输入账号和密码',true);return}
    submit.disabled=true;submit.textContent='登录中…';setStatus('正在验证客服账号…');
    try{
      const response=await fetch('/check',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded;charset=UTF-8'},body:new URLSearchParams({username:name,password:pass})});
      const payload=await response.json();
      if(!response.ok||payload.code!==200||!payload.result||!payload.result.token)throw new Error(payload.msg||payload.message||'账号或密码错误');
      localStorage.setItem('token',payload.result.token);localStorage.setItem('auth_username',name);location.reload();
    }catch(error){setStatus(error.message==='Failed to fetch'?'认证服务连接失败，请确认后端已启动':error.message||'登录失败',true);submit.disabled=false;submit.textContent='登 录'}
  });
  const token=localStorage.getItem('token')||'';
  if(!token){showGate('请输入客服账号和密码');return}
  setStatus('正在检查登录状态…');submit.disabled=true;
  fetch('/userinfo',{headers:{token}}).then(response=>response.json()).then(payload=>{
    if(payload.code===200){hideGate(payload.result&&(payload.result.nickname||payload.result.name));submit.disabled=false;return}
    clearAuth();showGate('登录已失效，请重新登录');submit.disabled=false;
  }).catch(()=>{showGate('认证服务连接失败，请确认后端已启动');submit.disabled=false});
})();
