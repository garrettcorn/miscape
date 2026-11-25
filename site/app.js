(function () {
  'use strict';
  var cfg = window.LANTERNFALL_SITE || {};
  var repo = cfg.repo;
  var gh = 'https://github.com/' + repo;
  var latest = gh + '/releases/latest/download/';
  var $ = function (s) { return document.querySelector(s); };

  var PLATFORMS = [
    { id: 'windows-amd64', os: 'windows', label: 'Windows (64-bit)' },
    { id: 'darwin-arm64', os: 'mac', label: 'macOS (Apple Silicon: M1 and newer)' },
    { id: 'darwin-amd64', os: 'mac', label: 'macOS (Intel)' },
    { id: 'linux-amd64', os: 'linux', label: 'Linux (x86-64)' },
    { id: 'linux-arm64', os: 'linux', label: 'Linux (ARM64, e.g. Raspberry Pi 4/5)' },
  ];
  var file = function (p) { return 'lanternfall-' + p.id + '.zip'; };

  function detect() {
    var ua = navigator.userAgent || '';
    var plat = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
    if (/Android|iPhone|iPad|iPod/i.test(ua)) return { mobile: true };
    if (/Win/i.test(plat) || /Windows/i.test(ua)) return { id: 'windows-amd64' };
    if (/Mac/i.test(plat) || /Mac OS X/i.test(ua)) return { id: 'darwin-arm64', guessed: true };
    if (/Linux/i.test(plat) || /Linux/i.test(ua)) return { id: /aarch64|arm/i.test(plat) ? 'linux-arm64' : 'linux-amd64' };
    return {};
  }

  function render(assets) {
    var has = function (p) { return !assets || assets.indexOf(file(p)) >= 0; };
    var d = detect();
    var rec = $('#recommended');
    var pick = PLATFORMS.filter(function (p) { return p.id === d.id; })[0];
    if (d.mobile) {
      rec.innerHTML = '<p class="muted">You\'re on a phone or tablet. The world server runs on a computer — see <a href="#play">Play in your browser</a> for ways to play here.</p>';
    } else if (pick && has(pick)) {
      rec.innerHTML = '<a class="btn primary big" href="' + latest + file(pick) + '">Download for ' + pick.label + '</a>' +
        (d.guessed ? '<p class="muted small">Older Intel Mac? Use the macOS (Intel) download below.</p>' : '');
      $('#dl-main').href = latest + file(pick);
      $('#dl-main').textContent = 'Download for ' + pick.label.replace(/ \(.*\)/, '');
    } else {
      rec.innerHTML = '';
    }
    $('#dl-table').innerHTML = PLATFORMS.map(function (p) {
      return '<tr><td>' + p.label + '</td><td>' + (has(p)
        ? '<a href="' + latest + file(p) + '">' + file(p) + '</a>'
        : '<span class="muted">not in this release</span>') + '</td></tr>';
    }).join('');
  }

  function noRelease() {
    $('#recommended').innerHTML = '<p class="note">No release has been published yet. Until then you can build from source: ' +
      '<code>go run ./cmd/lanternfall serve</code> (needs <a href="https://go.dev/dl/">Go</a>).</p>';
    $('#dl-main').href = '#download';
    $('#dl-main').textContent = 'Download';
    $('#dl-table').innerHTML = PLATFORMS.map(function (p) {
      return '<tr><td>' + p.label + '</td><td><span class="muted">available with the first release</span></td></tr>';
    }).join('');
  }

  $('#sums').href = latest + 'SHA256SUMS.txt';
  $('#all-releases').href = gh + '/releases';
  $('#ghlink').href = gh;
  Array.prototype.forEach.call(document.querySelectorAll('.doclink'), function (a) {
    a.href = gh + '/blob/main/' + a.getAttribute('data-path');
  });

  // Public world card.
  var pw = (cfg.publicWorldUrl || '').trim();
  if (/^https:\/\//.test(pw)) {
    $('#public-world').innerHTML = '<p>Jump straight in — nothing to install.</p><a class="btn primary" href="' + pw + '">Play now ↗</a>';
    var pm = $('#play-main');
    pm.href = pw;
    pm.textContent = 'Play now in your browser';
  }

  render(null);
  fetch('https://api.github.com/repos/' + repo + '/releases/latest', { headers: { Accept: 'application/vnd.github+json' } })
    .then(function (r) { if (!r.ok) throw new Error(r.status); return r.json(); })
    .then(function (rel) {
      var names = (rel.assets || []).map(function (a) { return a.name; });
      render(names);
      var date = rel.published_at ? new Date(rel.published_at).toLocaleDateString() : '';
      $('#dl-version').textContent = 'Latest: ' + rel.tag_name + (date ? ' (' + date + ')' : '') + '.';
      $('#version-line').textContent = 'Free · ' + rel.tag_name + ' · Windows, macOS & Linux · plays in any modern browser';
    })
    .catch(noRelease);
})();
