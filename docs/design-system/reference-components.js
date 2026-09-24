/* @ds-bundle: {"format":4,"namespace":"OpenChore","components":[{"name":"Greeting"},{"name":"Avatar"},{"name":"PointsChip"},{"name":"CategoryHeader"},{"name":"ChoreRow"},{"name":"DayProgress"},{"name":"Button"},{"name":"TabBar"},{"name":"Celebration"},{"name":"FamilyMember"},{"name":"Icon"},{"name":"DeviceFrame"}]} */
(function () {
  var React = window.React;
  var h = React.createElement;
  var ICONS = {"bed":"<path d=\"M3 18v-6.5A2.5 2.5 0 0 1 5.5 9h13a2.5 2.5 0 0 1 2.5 2.5V18M3 14.5h18M3 18v2M21 18v2\"/><path d=\"M6 9V7.5A1.5 1.5 0 0 1 7.5 6h3A1.5 1.5 0 0 1 12 7.5V9\"/>","tooth":"<path d=\"M8 3C5.5 3 4 4.9 4 7.2c0 2.3 1 3.6 1.6 5.6C6.3 15.2 6.5 21 8.5 21c1.7 0 1.6-5 3.5-5s1.8 5 3.5 5c2 0 2.2-5.8 2.9-8.2.6-2 1.6-3.3 1.6-5.6C20 4.9 18.5 3 16 3c-1.6 0-2.3 1-4 1S9.6 3 8 3z\"/>","paw":"<circle cx=\"6.5\" cy=\"10.5\" r=\"1.7\"/><circle cx=\"10\" cy=\"6.5\" r=\"1.7\"/><circle cx=\"14\" cy=\"6.5\" r=\"1.7\"/><circle cx=\"17.5\" cy=\"10.5\" r=\"1.7\"/><path d=\"M12 11.5c-2.6 0-5 3.2-5 5.6 0 1.6 1.2 2.4 2.6 2.4 1 0 1.6-.5 2.4-.5s1.4.5 2.4.5c1.4 0 2.6-.8 2.6-2.4 0-2.4-2.4-5.6-5-5.6z\"/>","dish":"<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><circle cx=\"12\" cy=\"12\" r=\"4.5\"/>","book":"<path d=\"M3.5 5.5c2.8-.9 5.6-.6 8.5 1.2v13c-2.9-1.8-5.7-2.1-8.5-1.2zM20.5 5.5c-2.8-.9-5.6-.6-8.5 1.2v13c2.9-1.8 5.7-2.1 8.5-1.2z\"/>","shirt":"<path d=\"M8.5 4 3.5 6.8l1.8 3.9 2.2-1V20h9V9.7l2.2 1 1.8-3.9L15.5 4c-.6 1.4-1.9 2.2-3.5 2.2S9.1 5.4 8.5 4z\"/>","sprout":"<path d=\"M12 20.5V12\"/><path d=\"M12 12c0-4 3-6.5 7.5-6.5 0 4.2-3 6.5-7.5 6.5zM12 14.5C12 11 9.5 9 5 9c0 3.4 2.5 5.5 7 5.5z\"/><path d=\"M7.5 20.5h9\"/>","toy":"<rect x=\"4\" y=\"11\" width=\"16\" height=\"9\" rx=\"2\"/><path d=\"M8 11V8.5a1.5 1.5 0 0 1 3 0V11M13 11V8.5a1.5 1.5 0 0 1 3 0V11\"/>","fish":"<path d=\"M3 12c2.5-4 6-5.5 9.5-5.5 3.5 0 6 2.5 7.5 5.5-1.5 3-4 5.5-7.5 5.5C9 17.5 5.5 16 3 12z\"/><path d=\"M3 12 1.5 9M3 12l-1.5 3\"/><circle cx=\"16\" cy=\"11\" r=\".6\" fill=\"currentColor\"/>","piano":"<rect x=\"3\" y=\"5\" width=\"18\" height=\"14\" rx=\"2\"/><path d=\"M8 19v-6M12 19v-6M16 19v-6M7 5v8h2V5M11 5v8h2V5M15 5v8h2V5\"/>","table":"<path d=\"M3 9.5h18M5 9.5 4 20M19 9.5l1 10.5M8.5 5.5v4M12 4v5.5M15.5 5.5v4\"/>","check":"<path d=\"M5 12.5l4.5 4.5L19 7.5\"/>","flame":"<path d=\"M12 21c3.9 0 7-2.8 7-6.6 0-3.4-2.4-5.4-3.6-8.4-.3 2-1.3 3.2-2.4 3.6C13 6 11.5 4 9 3c.4 3-1 5-2.4 6.8C5.8 11 5 12.4 5 14.4 5 18.2 8.1 21 12 21z\"/>","star":"<path d=\"M12 3.5l2.6 5.3 5.8.8-4.2 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8-4.2-4.1 5.8-.8z\"/>","spark":"<path d=\"M12 3c.7 5 3.9 8.2 9 9-5.1.8-8.3 4-9 9-.7-5-3.9-8.2-9-9 5.1-.8 8.3-4 9-9z\"/>","gift":"<rect x=\"4\" y=\"9\" width=\"16\" height=\"11\" rx=\"1.5\"/><path d=\"M3 9h18M12 9v11M12 9c-1.5-3-5.5-4-5.5-1.5S10 9 12 9zm0 0c1.5-3 5.5-4 5.5-1.5S14 9 12 9z\"/>","camera":"<path d=\"M4 8.5A1.5 1.5 0 0 1 5.5 7h2l1.5-2h6l1.5 2h2A1.5 1.5 0 0 1 20 8.5v9a1.5 1.5 0 0 1-1.5 1.5h-13A1.5 1.5 0 0 1 4 17.5z\"/><circle cx=\"12\" cy=\"13\" r=\"3.5\"/>","clock":"<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><path d=\"M12 7.5V12l3 2\"/>","home":"<path d=\"M4 11l8-7 8 7v8.5a1 1 0 0 1-1 1h-4.5V15h-5v5.5H5a1 1 0 0 1-1-1z\"/>","cal":"<rect x=\"4\" y=\"5\" width=\"16\" height=\"15\" rx=\"2.5\"/><path d=\"M4 10h16M8.5 3v4M15.5 3v4\"/>","sound":"<path d=\"M4 9.5h3.5L12 6v12l-4.5-3.5H4z\"/><path d=\"M15.5 9a4 4 0 0 1 0 6M18 6.5a7.5 7.5 0 0 1 0 11\"/>","lock":"<rect x=\"5\" y=\"10.5\" width=\"14\" height=\"10\" rx=\"2.5\"/><path d=\"M8 10.5V8a4 4 0 0 1 8 0v2.5\"/>","bike":"<circle cx=\"5.5\" cy=\"16\" r=\"3.5\"/><circle cx=\"18.5\" cy=\"16\" r=\"3.5\"/><path d=\"M5.5 16 9 9.5h6.5l3 6.5M9 9.5l3 6.5H5.5M14 6.5h2.5l-1 3\"/>","rocket":"<path d=\"M12 2.5c3 2.2 4.5 5.4 4.5 9.2V16h-9v-4.3c0-3.8 1.5-7 4.5-9.2z\"/><circle cx=\"12\" cy=\"9.5\" r=\"1.8\"/><path d=\"M7.5 12.5 5 15v3.5l2.5-2.5M16.5 12.5 19 15v3.5L16.5 16M10 19.5l2 2 2-2\"/>","film":"<rect x=\"3.5\" y=\"5\" width=\"17\" height=\"14\" rx=\"2.5\"/><path d=\"M3.5 9.5h17M8 5l2 4.5M13 5l2 4.5\"/>","bowl":"<path d=\"M3.5 11h17a8.5 8.5 0 0 1-17 0z\"/><path d=\"M9 7.5c0-1.5 1-2 1-3.5M13 7.5c0-1.5 1-2 1-3.5\"/>","moon":"<path d=\"M19.5 14.5A8 8 0 1 1 9.5 4.5a6.5 6.5 0 0 0 10 10z\"/>","cone":"<path d=\"M7 10.5h10L12 21.5z\"/><path d=\"M7 10.5a5 5 0 0 1 10 0\"/>","screen":"<rect x=\"3\" y=\"5\" width=\"18\" height=\"12\" rx=\"2\"/><path d=\"M9 20.5h6M12 17v3.5\"/>","back":"<path d=\"M14.5 6l-6 6 6 6\"/>","chev":"<path d=\"M9.5 6l6 6-6 6\"/>","plus":"<path d=\"M12 5v14M5 12h14\"/>","people":"<circle cx=\"9\" cy=\"8.5\" r=\"3.5\"/><path d=\"M2.5 20c.6-3.6 3.2-5.5 6.5-5.5s5.9 1.9 6.5 5.5\"/><path d=\"M15.5 5.2a3.5 3.5 0 0 1 0 6.6M18 14.8c2 .7 3.2 2.5 3.5 5.2\"/>","sb":"<rect x=\"0\" y=\"7\" width=\"3\" height=\"5\" rx=\"1\" fill=\"currentColor\"/><rect x=\"5\" y=\"5\" width=\"3\" height=\"7\" rx=\"1\" fill=\"currentColor\"/><rect x=\"10\" y=\"2.5\" width=\"3\" height=\"9.5\" rx=\"1\" fill=\"currentColor\"/><rect x=\"15\" y=\"0\" width=\"3\" height=\"12\" rx=\"1\" fill=\"currentColor\"/><path d=\"M26 4.2a9 9 0 0 1 12 0M28.2 6.8a5.6 5.6 0 0 1 7.6 0M30.4 9.4a2.2 2.2 0 0 1 3.2 0\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"1.6\" stroke-linecap=\"round\"/><rect x=\"43.5\" y=\".5\" width=\"20\" height=\"11\" rx=\"3\" fill=\"none\" stroke=\"currentColor\" opacity=\".45\"/><rect x=\"45.5\" y=\"2.5\" width=\"14\" height=\"7\" rx=\"1.6\" fill=\"currentColor\"/><rect x=\"64.5\" y=\"4\" width=\"1.5\" height=\"4\" rx=\".7\" fill=\"currentColor\" opacity=\".45\"/>"};

  function cx() { return Array.prototype.filter.call(arguments, Boolean).join(' '); }

  function Icon(p) {
    return h('svg', {
      className: cx('oc-icon', p.fill && 'oc-icon--fill', p.className),
      viewBox: '0 0 24 24', 'aria-hidden': true,
      style: p.size ? { width: p.size, height: p.size } : undefined,
      dangerouslySetInnerHTML: { __html: ICONS[p.name] || '' }
    });
  }

  var MARKS = {
    essential: '<circle cx="12" cy="12" r="9.5"/>',
    daily: '<rect x="2.5" y="2.5" width="19" height="19" rx="4.5"/>',
    bonus: '<path d="M12 1.8l3.1 6.4 7 1-5.1 4.9 1.2 7L12 17.8l-6.2 3.3 1.2-7-5.1-4.9 7-1z"/>'
  };
  var LABELS = { essential: 'Must do', daily: 'Every day', bonus: 'Bonus' };

  function CategoryMark(p) {
    return h('svg', {
      className: cx('oc-mark', 'oc-mark--' + p.cat, p.done && 'is-done'),
      viewBox: '0 0 24 24', 'aria-hidden': true,
      dangerouslySetInnerHTML: { __html: MARKS[p.cat] || MARKS.daily }
    });
  }

  function CategoryHeader(p) {
    return h('div', { className: 'oc-cathead' },
      h('span', { className: 'oc-cathead__l' }, h(CategoryMark, { cat: p.cat }), p.label || LABELS[p.cat]),
      p.count != null ? h('span', { className: 'oc-cathead__n' }, p.count) : null);
  }

  var BLOBS = [
    '58% 42% 52% 48% / 46% 56% 44% 54%',
    '44% 56% 40% 60% / 58% 42% 58% 42%',
    '52% 48% 62% 38% / 40% 52% 48% 60%',
    '48% 52% 44% 56% / 54% 46% 56% 44%'
  ];

  function Avatar(p) {
    var name = p.name || '?';
    return h('span', {
      className: cx('oc-avatar', 'oc-avatar--' + (p.size || 'md')),
      'data-person': p.color, title: name,
      style: { '--blob': BLOBS[name.charCodeAt(0) % BLOBS.length] }
    }, name.charAt(0));
  }

  function Greeting(p) {
    return h('h1', { className: 'oc-greeting' }, (p.salutation || 'Hi') + ', ', h('em', null, p.name + '.'));
  }

  function PointsChip(p) {
    return h('span', { className: 'oc-points', 'aria-label': p.points + ' points' }, h(Icon, { name: 'star' }), p.points);
  }

  function ChoreRow(p) {
    var st = p.state || 'todo';
    var meta = [];
    if (st === 'waiting') {
      meta.push(h('span', { key: 'w', className: 'oc-chore__waiting' }, p.meta || 'Waiting for a grown-up'));
    } else if (st === 'locked') {
      meta.push(p.meta || 'Opens when everything else is done');
    } else {
      if (p.photo) meta.push(h(Icon, { key: 'c', name: 'camera' }));
      var line = [];
      if (p.meta) line.push(p.urgent ? h('span', { key: 'u', className: 'oc-chore__urgent' }, p.meta) : p.meta);
      if (p.points != null) line.push((p.meta ? ' · ' : '') + p.points + ' pts');
      meta = meta.concat(line);
    }
    var check = st === 'done' ? h(Icon, { name: 'check' })
      : st === 'locked' ? h(Icon, { name: 'lock' })
      : null;
    return h('div', { className: cx('oc-chore', 'oc-chore--' + st), 'data-cat': p.cat || 'daily' },
      p.icon ? h('span', { className: 'oc-chore__well' }, h(Icon, { name: p.icon })) : null,
      h('span', { className: 'oc-chore__txt' },
        h('b', { className: 'oc-chore__title' }, p.title),
        meta.length ? h('span', { className: 'oc-chore__meta' }, meta) : null),
      p.readAloud && st !== 'done' ? h('span', { className: 'oc-chore__say', 'aria-label': 'Read aloud' }, h(Icon, { name: 'sound' })) : null,
      h('span', { className: 'oc-chore__check', role: 'checkbox', 'aria-checked': st === 'done' }, check));
  }

  function DayProgress(p) {
    var items = p.items || [];
    var done = items.filter(function (x) { return x.done; }).length;
    var total = items.length;
    var now = p.now == null ? 0.5 : p.now;
    function pt(f) { var t = Math.PI * (1 - f); return [156 + 144 * Math.cos(t), 92 - 80 * Math.sin(t)]; }
    var sun = pt(now);
    var ARC = 'M12 92 A144 80 0 0 1 300 92';
    var pins = items.filter(function (x) { return x.done && x.at != null; }).map(function (x, i) {
      var q = pt(x.at); return h('circle', { key: i, className: 'oc-prog__pin', cx: q[0], cy: q[1], r: 7 });
    });
    return h('div', { className: 'oc-prog', 'aria-label': done + ' of ' + total + ' done today' },
      h('div', { className: 'oc-prog__arcwrap' },
        h('svg', { className: 'oc-prog__arc', viewBox: '0 0 312 100', 'aria-hidden': true },
          h('path', { className: 'oc-prog__track', d: ARC }),
          h('path', { className: 'oc-prog__fill', d: ARC, pathLength: 100, strokeDasharray: (now * 100) + ' 100' }),
          pins,
          h('circle', { className: 'oc-prog__halo', cx: sun[0], cy: sun[1], r: 20 }),
          h('circle', { className: 'oc-prog__sun', cx: sun[0], cy: sun[1], r: 11 })),
        h('div', { className: 'oc-prog__arcnum' }, h('b', null, done, h('span', null, ' of ' + total)), h('small', null, 'done today')),
        h('div', { className: 'oc-prog__ends' }, h('span', null, p.from || '7 am'), h('span', null, p.to || '9 pm'))),
      h('div', { className: 'oc-prog__shapes' },
        items.map(function (x, i) { return h(CategoryMark, { key: i, cat: x.cat, done: x.done }); }),
        h('b', null, done + ' of ' + total)),
      h('div', { className: 'oc-prog__ringwrap' },
        h('svg', { className: 'oc-prog__ring', viewBox: '0 0 212 212', 'aria-hidden': true },
          h('circle', { className: 'oc-prog__rtrack', cx: 106, cy: 106, r: 90 }),
          h('circle', { className: 'oc-prog__rfill', cx: 106, cy: 106, r: 90, pathLength: 100, strokeDasharray: (total ? done / total * 100 : 0) + ' 100' })),
        h('svg', { className: 'oc-prog__spark', viewBox: '0 0 24 24', style: { right: -6, top: 6, width: 20, height: 20 }, dangerouslySetInnerHTML: { __html: ICONS.spark } }),
        h('svg', { className: 'oc-prog__spark', viewBox: '0 0 24 24', style: { right: 18, top: -10, width: 11, height: 11, opacity: 0.7 }, dangerouslySetInnerHTML: { __html: ICONS.spark } }),
        h('svg', { className: 'oc-prog__spark', viewBox: '0 0 24 24', style: { left: -14, bottom: 30, width: 13, height: 13, opacity: 0.5 }, dangerouslySetInnerHTML: { __html: ICONS.spark } }),
        h('div', { className: 'oc-prog__ringnum' }, h('b', null, done, h('span', null, '/' + total)), h('small', null, 'done today'))));
  }

  function Button(p) {
    return h('button', { type: 'button', className: cx('oc-btn', 'oc-btn--' + (p.variant || 'primary'), p.block && 'oc-btn--block'), onClick: p.onClick },
      p.children, p.icon ? h(Icon, { name: p.icon }) : null);
  }

  var TABS = [['today', 'Today', 'home'], ['week', 'Week', 'cal'], ['rewards', 'Rewards', 'gift']];
  function TabBar(p) {
    var active = p.active || 'today';
    return h('nav', { className: 'oc-tabs' }, TABS.map(function (t) {
      return h('span', { key: t[0], className: cx('oc-tab', t[0] === active && 'is-on') },
        h('span', { className: 'oc-tab__pill' }, h(Icon, { name: t[2] }), h('span', { className: 'oc-tab__label' }, t[1])),
        h('span', { className: 'oc-tab__dot' }));
    }));
  }

  function Celebration(p) {
    return h('div', { className: 'oc-cele' },
      h('svg', { className: 'oc-cele__rays', viewBox: '0 0 360 712', preserveAspectRatio: 'xMidYMin slice', 'aria-hidden': true },
        h('circle', { className: 'sun', cx: 300, cy: 70, r: 120 }),
        h('circle', { className: 'dash', cx: 300, cy: 70, r: 150 }),
        h('circle', { className: 'dash', cx: 300, cy: 70, r: 182 }),
        null),
      h('svg', { className: 'oc-cele__confetti', viewBox: '0 0 360 712', 'aria-hidden': true },
        h('circle', { className: 'c1', cx: 40, cy: 50, r: 34 }),
        h('rect', { className: 'c2', x: 262, y: 20, width: 64, height: 64, rx: 12, transform: 'rotate(18 294 52)' }),
        h('path', { className: 'c3', d: 'M300 290l38 66h-76z', transform: 'rotate(-14 300 330)' }),
        h('rect', { className: 'c4', x: -20, y: 290, width: 86, height: 34, rx: 17, transform: 'rotate(-24 23 307)' }),
        h('path', { className: 'sq', d: 'M150 64c10-12 20 12 30 0s20 12 30 0' }),
        h('circle', { className: 'c5', cx: 318, cy: 200, r: 12 }),
        h('rect', { className: 'c1', x: 18, y: 520, width: 46, height: 46, rx: 10, transform: 'rotate(-12 41 543)' }),
        h('circle', { className: 'c2', cx: 330, cy: 520, r: 22 }),
        h('path', { className: 'c5', d: 'M82 480l7 15 16 2-12 11 3 16-14-8-14 8 3-16-12-11 16-2z' })),
      h('svg', { className: 'oc-cele__ringdeco', viewBox: '0 0 360 712', preserveAspectRatio: 'xMidYMin slice', 'aria-hidden': true },
        h('circle', { className: 'glow', cx: 180, cy: 330, r: 190 }),
        h('circle', { className: 'trk', cx: 180, cy: 330, r: 150 }),
        h('circle', { className: 'arc', cx: 180, cy: 330, r: 150, pathLength: 100, strokeDasharray: '100 100' }),
        h('path', { className: 'sp', d: 'M318 150c.7 5 3.9 8.2 9 9-5.1.8-8.3 4-9 9-.7-5-3.9-8.2-9-9 5.1-.8 8.3-4 9-9z' }),
        h('path', { className: 'sp', d: 'M40 470c.5 3.6 2.8 5.9 6.5 6.5-3.7.6-6 2.9-6.5 6.5-.5-3.6-2.8-5.9-6.5-6.5 3.7-.6 6-2.9 6.5-6.5z' })),
      h('div', { className: 'oc-cele__head' }, p.headline || 'Nailed ', p.headline ? null : h('em', null, 'it!')),
      h('div', { className: 'oc-cele__card' },
        h('div', { className: 'oc-cele__pts' }, '+' + p.points),
        h('div', { className: 'oc-cele__what' }, p.title),
        p.streak ? h('div', { className: 'oc-cele__streak' }, h(Icon, { name: 'flame' }), p.streak + ' in a row') : null),
      h('div', { className: 'oc-cele__foot' },
        p.next ? h(Button, { block: true, icon: 'chev' }, 'Next: ' + p.next) : null,
        h(Button, { variant: 'quiet' }, 'Back to my list')));
  }

  function FamilyMember(p) {
    var pct = p.total ? Math.round(p.done / p.total * 100) : 0;
    return h('div', { className: 'oc-member', 'data-person': p.color },
      h(Avatar, { name: p.name, color: p.color, size: 'sm' }),
      h('span', { className: 'oc-member__n' }, p.name),
      h('span', { className: 'oc-member__bar' }, h('i', { style: { width: pct + '%' } })),
      h('span', { className: 'oc-member__c' }, p.done + '/' + p.total));
  }

  function DeviceFrame(p) {
    return h('div', { className: 'oc-device', 'data-theme': p.theme, 'data-person': p.person, style: p.style },
      h('div', { className: 'oc-device__glow' }),
      h('div', { className: 'oc-status' }, h('span', null, p.time || '9:41'),
        h('svg', { viewBox: '0 0 66 12', 'aria-hidden': true, dangerouslySetInnerHTML: { __html: ICONS.sb } })),
      h('div', { className: 'oc-device__body' }, p.children));
  }

  window.OpenChore = {
    Greeting: Greeting, Avatar: Avatar, PointsChip: PointsChip, CategoryHeader: CategoryHeader,
    CategoryMark: CategoryMark, ChoreRow: ChoreRow, DayProgress: DayProgress, Button: Button,
    TabBar: TabBar, Celebration: Celebration, FamilyMember: FamilyMember, Icon: Icon,
    DeviceFrame: DeviceFrame, ICON_NAMES: Object.keys(ICONS).filter(function (k) { return k !== 'sb'; })
  };
})();
