// Dev-only gallery at /design: every component in every theme, plus
// reconstructions of the reference screens (docs/design-system/screens).
// Demo data is intentionally hard-coded English.
import React from 'react';
import {
  Avatar, Button, CategoryHeader, Celebration, ChoreRow, DayProgress, FamilyMember, Greeting, HouseScope,
  Icon, ICON_NAMES, PERSON_COLORS, PointsChip, SkinScope, TabBar,
  type DayProgressItem, type PersonColor, type Skin, type ThemeId, type Cat,
} from '..';
import './gallery.css';

const SKIN_NOTES: Record<Skin, string> = {
  sunroom: 'Sunroom — sun arc, serif, soft cards',
  blocks: 'Blocks — shapes, outlines, press-flat blocks',
  tint: "Tint — one ring, lit in Lily's mint",
};
const SKINS: Skin[] = ['sunroom', 'blocks', 'tint'];
const ALL_THEMES: ThemeId[] = ['sunroom', 'blocks', 'tint', 'house', 'house-dark'];
const THEME_NAMES: Record<ThemeId, string> = {
  sunroom: 'Sunroom', blocks: 'Blocks', tint: 'Tint', house: 'House', 'house-dark': 'House Dark',
};

const LILY_DAY: DayProgressItem[] = [
  { cat: 'essential', done: true, at: 0.04 }, { cat: 'daily', done: true, at: 0.1 },
  { cat: 'essential', done: true, at: 0.17 }, { cat: 'daily', done: true, at: 0.33 },
  { cat: 'daily', done: true, at: 0.44 }, { cat: 'daily', done: true, at: 0.52 },
  { cat: 'daily' }, { cat: 'bonus' },
];

function StatusBar({ time }: { time: string }) {
  return (
    <div className="g-status">
      <span>{time}</span>
      <svg viewBox="0 0 66 12" aria-hidden>
        <rect x="0" y="7" width="3" height="5" rx="1" fill="currentColor" />
        <rect x="5" y="5" width="3" height="7" rx="1" fill="currentColor" />
        <rect x="10" y="2.5" width="3" height="9.5" rx="1" fill="currentColor" />
        <rect x="15" y="0" width="3" height="12" rx="1" fill="currentColor" />
        <path d="M26 4.2a9 9 0 0 1 12 0M28.2 6.8a5.6 5.6 0 0 1 7.6 0M30.4 9.4a2.2 2.2 0 0 1 3.2 0" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        <rect x="43.5" y=".5" width="20" height="11" rx="3" fill="none" stroke="currentColor" opacity=".45" />
        <rect x="45.5" y="2.5" width="14" height="7" rx="1.6" fill="currentColor" />
        <rect x="64.5" y="4" width="1.5" height="4" rx=".7" fill="currentColor" opacity=".45" />
      </svg>
    </div>
  );
}

/** A phone frame for the gallery only (the real app is the screen itself). */
function Device({ skin, person, time, ground, children }: { skin: Skin; person?: PersonColor; time: string; ground?: string; children: React.ReactNode }) {
  return (
    <SkinScope skin={skin} color={person} className="g-device" style={ground ? { background: ground } : undefined}>
      <StatusBar time={time} />
      <div className="g-device__body">{children}</div>
    </SkinScope>
  );
}

function Today() {
  const [bed, setBed] = React.useState(true);
  return (
    <>
      <div className="g-pad">
        <div className="g-top">
          <span className="g-me"><Avatar name="Lily" color="mint" size="md" />Thursday</span>
          <PointsChip points={185} />
        </div>
        <div style={{ marginTop: 16 }}><Greeting salutation="Afternoon" name="Lily" /></div>
        <div style={{ marginTop: 12 }}><DayProgress items={LILY_DAY} now={0.66} /></div>
        <CategoryHeader cat="essential" count="2 of 2" />
        <ChoreRow cat="essential" icon="🧸" title="Tidy your room" meta="Waiting for Mom to check" state="waiting" />
        <ChoreRow cat="essential" icon="🛏️" title="Make your bed" meta="Done at 7:32 am" state={bed ? 'done' : 'todo'} onToggle={() => setBed((b) => !b)} />
        <CategoryHeader cat="daily" count="4 of 5" />
        <ChoreRow cat="daily" icon="piano" title="Practice piano" meta="44 min left" urgent points={10} />
        <ChoreRow cat="daily" icon="shirt" title="Fold your laundry" meta="Photo check" photo points={10} />
        <CategoryHeader cat="bonus" count="+15" />
        <ChoreRow cat="bonus" icon="sprout" title="Water the garden" state="locked" />
      </div>
      <div className="g-fade" />
      <TabBar active="today" />
    </>
  );
}

function TodayScreens() {
  return (
    <div className="g-row g-row--devices">
      {SKINS.map((s, i) => (
        <figure key={s} className="g-fig">
          <Device skin={s} person="mint" time="4:15"><Today /></Device>
          <figcaption className="g-cap"><b>0{i + 1}</b> {SKIN_NOTES[s]}</figcaption>
        </figure>
      ))}
    </div>
  );
}

function CelebrationScreens() {
  return (
    <div className="g-row g-row--devices">
      {SKINS.map((s) => (
        <figure key={s} className="g-fig">
          <Device skin={s} person="mint" time="4:18" ground="var(--celebrate)">
            <Celebration points={10} title="Unload the dishwasher" streak={4} next="Feed the cats" />
          </Device>
          <figcaption className="g-cap"><b>{THEME_NAMES[s]}</b> Same component, this skin's dials</figcaption>
        </figure>
      ))}
    </div>
  );
}

/* ---------------- Family picker ---------------- */

function day(done: number, total: number, cats: string, now: number) {
  const at = [0.04, 0.1, 0.17, 0.3, 0.4, 0.5, 0.56, 0.6, 0.63, 0.8, 0.9];
  const map: Record<string, Cat> = { e: 'essential', d: 'daily', b: 'bonus' };
  return {
    now,
    items: cats.slice(0, total).split('').map((c, i): DayProgressItem => ({ cat: map[c], done: i < done, at: at[i] })),
  };
}

interface Person { name: string; color: PersonColor; skin: Skin; left: string; pts: number; d: ReturnType<typeof day> }
const MODES: {
  mode: 'light' | 'dark'; label: string; note: string; time: string; hello: string; check: string; gift: string;
  people: Person[]; fam: number[];
}[] = [
  {
    mode: 'light', label: 'Day · House', note: 'Light on shared devices by day, and whenever a personal device is set to light.',
    time: 'Thursday, September 24 · 4:15 pm', hello: 'Good afternoon.', check: '2 chores to check', gift: '1 reward to hand out',
    people: [
      { name: 'Emma', color: 'coral', skin: 'sunroom', left: '3 to go', pts: 240, d: day(6, 9, 'eeddddddb', 0.66) },
      { name: 'Lily', color: 'mint', skin: 'tint', left: '2 to go', pts: 185, d: day(6, 8, 'eedddddb', 0.66) },
      { name: 'Noah', color: 'butter', skin: 'blocks', left: '4 to go', pts: 85, d: day(3, 7, 'edeeddb', 0.66) },
    ],
    fam: [6, 9, 6, 8, 3, 7, 2, 3],
  },
  {
    mode: 'dark', label: 'Night · House Dark', note: 'The wall tablet switches at 7:30 pm; phones follow their system setting. Doors dim with it, so no skin glares in a dark hallway.',
    time: 'Thursday, September 24 · 8:40 pm', hello: 'Good evening.', check: '1 chore to check', gift: 'Nothing to hand out',
    people: [
      { name: 'Emma', color: 'coral', skin: 'sunroom', left: 'All done', pts: 275, d: day(9, 9, 'eeddddddb', 0.97) },
      { name: 'Lily', color: 'mint', skin: 'tint', left: '1 to go', pts: 205, d: day(7, 8, 'eedddddb', 0.97) },
      { name: 'Noah', color: 'butter', skin: 'blocks', left: 'All done', pts: 120, d: day(7, 7, 'edeeddb', 0.97) },
    ],
    fam: [9, 9, 7, 8, 7, 7, 3, 3],
  },
];

// Door hero sizing is layout, not theme: the gallery passes it per door.
const DOOR_HERO: Record<Skin, React.CSSProperties> = {
  sunroom: { '--door-scale': 0.8, marginTop: 40 } as React.CSSProperties,
  blocks: { '--door-scale': 0.92, width: 262 } as React.CSSProperties,
  tint: { '--door-scale': 0.8 } as React.CSSProperties,
};

function Door({ p }: { p: Person }) {
  return (
    <SkinScope skin={p.skin} color={p.color} door className="fp-door">
      <Avatar name={p.name} color={p.color} size="lg" />
      <div className="fp-name">{p.name}</div>
      <div className="fp-meta">
        <span className={p.left === 'All done' ? 'fp-done' : undefined}>{p.left}</span> · <Icon name="star" />{p.pts}
      </div>
      <div className="fp-heroz">
        <div className="fp-prog" style={DOOR_HERO[p.skin]}>
          <DayProgress items={p.d.items} now={p.d.now} />
        </div>
      </div>
    </SkinScope>
  );
}

function FamilyPicker() {
  return (
    <div className="fp-all">
      {MODES.map((m) => {
        const f = m.fam;
        return (
          <figure key={m.mode} className="fp-stage">
            <HouseScope mode={m.mode} className="fp-tab">
              <div className="fp-top">
                <div className="fp-brand">
                  <span className="fp-logo">
                    {(['coral', 'mint', 'butter', 'sky'] as const).map((c) => <i key={c} style={{ background: `var(--person-${c})` }} />)}
                  </span>
                  openchore
                </div>
                <div className="fp-date">{m.time}</div>
              </div>
              <h1 className="fp-hero">{m.hello} <span>Who's here?</span></h1>
              <div className="fp-doors">
                {m.people.map((p) => <Door key={p.name} p={p} />)}
                <div className="fp-door fp-grown">
                  <span className="fp-stack">
                    <Avatar name="Alex" color="sky" size="lg" />
                    <Avatar name="Jamie" size="lg" />
                  </span>
                  <div className="fp-name"><Icon name="lock" />Grown-ups</div>
                  <div className="fp-meta">PIN or linked account</div>
                  <div className="fp-todo">
                    <span className="w"><Icon name="clock" />{m.check}</span>
                    <span><Icon name="gift" />{m.gift}</span>
                  </div>
                </div>
              </div>
              <div className="fp-fam">
                <h3>The family<br />today</h3>
                <FamilyMember name="Emma" color="coral" done={f[0]} total={f[1]} />
                <FamilyMember name="Lily" color="mint" done={f[2]} total={f[3]} />
                <FamilyMember name="Noah" color="butter" done={f[4]} total={f[5]} />
                <FamilyMember name="Alex" color="sky" done={f[6]} total={f[7]} />
              </div>
            </HouseScope>
            <figcaption className="g-cap"><b>{m.label}</b> {m.note}</figcaption>
          </figure>
        );
      })}
    </div>
  );
}

/* ---------------- Component matrix ---------------- */

function ThemePanel({ theme }: { theme: ThemeId }) {
  const [done, setDone] = React.useState(false);
  const body = (
    <>
      <h3 className="g-label">{THEME_NAMES[theme]}</h3>

      <Greeting salutation="Afternoon" name="Lily" />

      <div className="g-line">
        <Avatar name="Emma" color="coral" size="lg" />
        <Avatar name="Lily" color="mint" size="lg" />
        <Avatar name="Noah" color="butter" size="lg" />
      </div>
      <div className="g-line g-wrap">
        {PERSON_COLORS.map((c, i) => <Avatar key={c} name={'ELNARFIS'[i]} color={c} size="md" />)}
        <Avatar name="Jamie" size="sm" />
      </div>

      <div className="g-line">
        <PointsChip points={185} />
        <Button icon="chev">Next</Button>
        <Button variant="quiet">Back</Button>
      </div>

      <CategoryHeader cat="essential" count="2 of 2" />
      <ChoreRow cat="essential" icon="🐱" title="Feed the cats" meta="44 min left" urgent points={5} state={done ? 'done' : 'todo'} onToggle={() => setDone((d) => !d)} />
      <ChoreRow cat="essential" icon="bed" title="Make your bed" meta="Done at 7:32 am" state="done" />
      <CategoryHeader cat="daily" count="3 of 5" />
      <ChoreRow cat="daily" icon="🍽️" title="Unload the dishwasher" meta="Snap a photo" photo points={10} />
      <ChoreRow cat="daily" icon="toy" title="Tidy your room" meta="Waiting for Mom to check" state="waiting" />
      <ChoreRow cat="daily" icon="🪥" title="Brush your teeth" meta="Before 9:00" points={5} readAloud />
      <CategoryHeader cat="bonus" count="+15" />
      <ChoreRow cat="bonus" icon="🌱" title="Water the garden" state="locked" />
      <ChoreRow cat="bonus" icon="🗑️" title="Take out the bins" points={15} />

      <div style={{ marginTop: 24 }}><DayProgress items={LILY_DAY} now={0.66} /></div>

      <div className="g-tabs"><TabBar active="today" position="static" /></div>

      <div>
        <FamilyMember name="Emma" color="coral" done={6} total={9} />
        <FamilyMember name="Lily" color="mint" done={6} total={8} />
        <FamilyMember name="Rosa" color="rose" done={2} total={5} />
        <FamilyMember name="Finn" color="leaf" done={4} total={4} />
        <FamilyMember name="Ivy" color="lilac" done={1} total={6} />
        <FamilyMember name="Sam" color="sand" done={3} total={7} />
      </div>

      <div className="g-icons">
        {ICON_NAMES.map((n) => (
          <span key={n} className="g-icon" title={n}><Icon name={n} /><small>{n}</small></span>
        ))}
      </div>
    </>
  );
  if (theme === 'house' || theme === 'house-dark') {
    return <HouseScope mode={theme === 'house' ? 'light' : 'dark'} className="g-panel">{body}</HouseScope>;
  }
  return <SkinScope skin={theme} color="mint" className="g-panel">{body}</SkinScope>;
}

export default function DesignGallery() {
  return (
    <HouseScope mode="light" className="g-page">
      <header className="g-head">
        <h1>OpenChore design system</h1>
        <p>Dev-only gallery. Every component below is the same code; only <code>data-theme</code> / <code>data-person</code> change.</p>
        <nav className="g-nav">
          <a href="#today">Today in three skins</a>
          <a href="#picker">Family picker</a>
          <a href="#components">Components</a>
          <a href="#celebration">Celebration</a>
        </nav>
      </header>

      <section id="today" className="g-section">
        <h2>Today in three skins</h2>
        <TodayScreens />
      </section>

      <section id="picker" className="g-section">
        <h2>Family picker</h2>
        <FamilyPicker />
      </section>

      <section id="components" className="g-section">
        <h2>Components in every theme</h2>
        <div className="g-matrix">
          {ALL_THEMES.map((t) => <ThemePanel key={t} theme={t} />)}
        </div>
      </section>

      <section id="celebration" className="g-section">
        <h2>Celebration</h2>
        <CelebrationScreens />
      </section>
    </HouseScope>
  );
}
