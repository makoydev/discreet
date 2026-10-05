(function () {
  'use strict';
  var $ = function (id) { return document.getElementById(id); };
  var root = document.documentElement;

  // Theme toggle, remembered per browser.
  try { var t = localStorage.getItem('discreet-theme'); if (t === 'light' || t === 'dark') root.setAttribute('data-theme', t); } catch (e) {}
  function theme() { return root.getAttribute('data-theme') || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'); }
  function label() { $('theme').textContent = theme() === 'dark' ? 'Light theme' : 'Dark theme'; }
  label();
  $('theme').addEventListener('click', function () {
    var next = theme() === 'dark' ? 'light' : 'dark';
    root.setAttribute('data-theme', next);
    try { localStorage.setItem('discreet-theme', next); } catch (e) {}
    label();
  });

  // Made-up examples. S1234567D and T0000001E have valid check letters but
  // are patterned, synthetic numbers; the card is a published test number.
  var EXAMPLES = [
    { purpose: 'claims summary', text: 'Member S1234567D called from 9123 4567 about claim C-2041. Please draft a short reply to tan.ah.kow@example.com explaining the next steps. Copy her husband, T0000001E.' },
    { purpose: 'appointment reminder', text: 'Remind the patient (DOB: 12/03/1988) at Blk 123 Ang Mo Kio Avenue 3 #05-123, Singapore 560123, that her check-up is on Friday at 10am. Mobile +65 9876 5432.' },
    { purpose: 'card dispute reply', text: 'A customer disputes a charge of SGD 84.20 on card 4111 1111 1111 1111. Draft a reply to lim.mei@example.com saying we are investigating.' },
    { purpose: 'ticket triage', text: 'Ticket from S1234567A: "I typed my NRIC wrong on the form, my number is S1234567D, please fix it." Classify the ticket.' },
    { purpose: 'automated eligibility decision', text: 'Decide whether S1234567D is eligible for the household support scheme and reply yes or no.' }
  ];
  var next = 0;
  function load() {
    var ex = EXAMPLES[next % EXAMPLES.length];
    next++;
    $('purpose').value = ex.purpose;
    $('text').value = ex.text;
  }
  load();
  $('example').addEventListener('click', function () { load(); run(); });

  var TOKEN = /(<[A-Z][A-Z_]*_\d+>|\[REDACTED_[A-Z_]+\])/;
  function placeholders(el, text, keep) {
    if (!keep) el.textContent = '';
    text.split(TOKEN).forEach(function (piece) {
      if (!piece) return;
      if (TOKEN.test(piece) && piece.match(TOKEN)[0] === piece) {
        var s = document.createElement('span');
        s.className = 'ph' + (piece.charAt(0) === '[' ? ' redacted' : '');
        s.textContent = piece;
        el.appendChild(s);
      } else {
        el.appendChild(document.createTextNode(piece));
      }
    });
  }
  function empty(el, text) {
    el.textContent = '';
    var s = document.createElement('span'); s.className = 'empty'; s.textContent = text; el.appendChild(s);
  }

  function render(out) {
    $('p1').textContent = out.original;
    $('chips').textContent = '';
    var rec = out.audit_record;
    if (out.status === 'refused') {
      $('status').className = 'status refused';
      $('status').textContent = 'Refused: ' + out.message;
      var box = document.createElement('div'); box.className = 'refusal'; box.textContent = out.message;
      $('p2').textContent = ''; $('p2').appendChild(box);
      empty($('p3'), 'The model was never called.');
      empty($('p4'), 'The app received an error (' + out.code + ') instead of an answer.');
      chip(rec ? rec.decision : out.code, true);
    } else {
      $('status').className = 'status';
      $('status').textContent = 'Done. Compare pane 1 with pane 2: the AI never saw the real details.';
      placeholders($('p2'), out.sent_to_model);
      placeholders($('p3'), out.model_answer);
      var p4 = $('p4'); p4.textContent = '';
      out.returned.forEach(function (part, i) {
        if (part.placeholder) {
          var s = document.createElement('span'); s.className = 'restored'; s.title = 'Put back in place of ' + part.placeholder; s.textContent = part.text;
          p4.appendChild(s);
        } else if (i === out.returned.length - 1 && part.text.indexOf('Personal data was replaced') !== -1) {
          var f = document.createElement('span'); f.className = 'footer-note'; f.textContent = part.text; p4.appendChild(f);
        } else {
          placeholders(p4, part.text, true); // marks anything never restored, such as [REDACTED_CARD]
        }
      });
      chip(rec.decision, false);
    }
    if (rec) {
      Object.keys(rec.entities || {}).sort().forEach(function (k) { chip(k + ' × ' + rec.entities[k], false); });
      $('record').textContent = JSON.stringify(rec, null, 2);
    } else {
      $('record').textContent = '(no record: the request was rejected before it was processed)';
    }
  }
  function chip(text, bad) {
    var c = document.createElement('span'); c.className = 'chip' + (bad ? ' refused' : ''); c.textContent = text; $('chips').appendChild(c);
  }

  function run() {
    var text = $('text').value;
    if (!text.trim()) { $('status').textContent = 'Type or load a message first.'; return; }
    $('send').disabled = true;
    $('status').className = 'status';
    $('status').textContent = 'Sending…';
    fetch('demo/run', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: text, purpose: $('purpose').value }) })
      .then(function (r) { return r.json().then(function (body) { return { ok: r.ok || r.status === 403, status: r.status, body: body }; }); })
      .then(function (res) {
        if (res.body && res.body.original !== undefined) { render(res.body); return; }
        var msg = res.body && res.body.error ? res.body.error.message : 'Something went wrong (' + res.status + ').';
        $('status').className = 'status refused'; $('status').textContent = msg;
      })
      .catch(function () { $('status').className = 'status refused'; $('status').textContent = 'Could not reach Discreet.'; })
      .finally(function () { $('send').disabled = false; });
  }
  $('send').addEventListener('click', run);
  empty($('p1'), 'Load an example or type a message, then send it.');
  ['p2', 'p3', 'p4'].forEach(function (id) { empty($(id), 'Waiting for a message.'); });
  $('record').textContent = '';
})();
