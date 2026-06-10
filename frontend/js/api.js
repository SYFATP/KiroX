// Wails/Web 统一调用层
(function() {
  function hasWailsApp() {
    return !!(window.go && window.go.main && window.go.main.App);
  }

  function isWeb() {
    return !hasWailsApp();
  }

  async function request(method, args) {
    var res = await fetch('/api/' + method, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(args || [])
    });
    if (res.status === 401) {
      showLoginOverlay();
      throw new Error('unauthorized');
    }
    if (!res.ok) {
      var text = await res.text();
      throw new Error(text || ('HTTP ' + res.status));
    }
    return await res.json();
  }

  var api = new Proxy({
    isWeb: isWeb,
    isDesktop: function() { return hasWailsApp(); },
    openURL: function(url) {
      if (hasWailsApp() && window.go.main.App.OpenURL) return window.go.main.App.OpenURL(url);
      window.open(url, '_blank', 'noopener,noreferrer');
      return Promise.resolve();
    },
    environment: async function() {
      if (window.runtime && window.runtime.Environment) return await window.runtime.Environment();
      return { platform: 'browser' };
    },
    login: async function(password) {
      var res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({ password: password })
      });
      if (!res.ok) throw new Error('登录失败');
      hideLoginOverlay();
      return await res.json();
    },
    session: async function() {
      if (hasWailsApp()) return { authRequired: false, authenticated: true };
      var res = await fetch('/api/session', { credentials: 'same-origin' });
      return await res.json();
    },
    selectLocalTextFile: function() {
      return new Promise(function(resolve, reject) {
        var input = document.createElement('input');
        input.type = 'file';
        input.accept = '.txt,.csv,text/plain,text/csv';
        input.style.display = 'none';
        input.onchange = function() {
          var file = input.files && input.files[0];
          document.body.removeChild(input);
          if (!file) return resolve('');
          var reader = new FileReader();
          reader.onload = function() { resolve(String(reader.result || '')); };
          reader.onerror = function() { reject(reader.error || new Error('读取文件失败')); };
          reader.readAsText(file);
        };
        document.body.appendChild(input);
        input.click();
      });
    }
  }, {
    get: function(target, prop) {
      if (prop in target) return target[prop];
      return async function() {
        var args = Array.prototype.slice.call(arguments);
        if (hasWailsApp() && window.go.main.App[prop]) {
          return await window.go.main.App[prop].apply(window.go.main.App, args);
        }
        return await request(prop, args);
      };
    }
  });

  function ensureLoginOverlay() {
    var existing = document.getElementById('web-login-overlay');
    if (existing) return existing;
    var el = document.createElement('div');
    el.id = 'web-login-overlay';
    el.style.cssText = 'display:none;position:fixed;inset:0;z-index:10000;background:rgba(15,23,42,.72);backdrop-filter:blur(8px);align-items:center;justify-content:center;padding:24px;';
    el.innerHTML = '' +
      '<div style="width:100%;max-width:360px;background:var(--card-bg,#fff);border:1px solid var(--border,#e5e7eb);border-radius:16px;padding:28px;box-shadow:0 24px 80px rgba(0,0,0,.28);">' +
        '<div style="font-size:20px;font-weight:800;color:var(--text,#111827);margin-bottom:6px;">KiroX Web</div>' +
        '<div style="font-size:13px;color:var(--text-muted,#6b7280);margin-bottom:18px;">请输入访问密码</div>' +
        '<input id="web-login-password" type="password" class="form-input" placeholder="Password" style="width:100%;box-sizing:border-box;margin-bottom:12px;">' +
        '<button id="web-login-btn" class="btn btn-dark" style="width:100%;justify-content:center;">登录</button>' +
        '<div id="web-login-error" style="display:none;margin-top:12px;color:var(--danger,#ef4444);font-size:12px;"></div>' +
      '</div>';
    document.body.appendChild(el);
    var submit = async function() {
      var pwd = document.getElementById('web-login-password').value;
      var err = document.getElementById('web-login-error');
      err.style.display = 'none';
      try { await api.login(pwd); }
      catch (e) { err.textContent = e.message || '登录失败'; err.style.display = 'block'; }
    };
    el.querySelector('#web-login-btn').addEventListener('click', submit);
    el.querySelector('#web-login-password').addEventListener('keydown', function(e) { if (e.key === 'Enter') submit(); });
    return el;
  }

  function showLoginOverlay() {
    if (!isWeb()) return;
    ensureLoginOverlay().style.display = 'flex';
    setTimeout(function() {
      var input = document.getElementById('web-login-password');
      if (input) input.focus();
    }, 0);
  }

  function hideLoginOverlay() {
    var el = document.getElementById('web-login-overlay');
    if (el) el.style.display = 'none';
  }

  window.AppAPI = api;
  window.showLoginOverlay = showLoginOverlay;
  window.hideLoginOverlay = hideLoginOverlay;

  window.addEventListener('DOMContentLoaded', async function() {
    if (!isWeb()) return;
    document.body.classList.add('web-mode');
    try {
      var s = await api.session();
      if (s.authRequired && !s.authenticated) showLoginOverlay();
    } catch (e) {
      showLoginOverlay();
    }
  });
})();
