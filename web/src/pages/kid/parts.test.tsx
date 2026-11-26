// @vitest-environment jsdom
import { describe, it, expect, beforeAll, afterEach } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import i18n from '../../i18n';
import type { ScheduledChore } from '../../types';
import { localDateStr } from '../../utils';
import { choreView } from './choreView';
import { ChoreGroups } from './parts';

let id = 0;
function chore(over: Partial<ScheduledChore>): ScheduledChore {
  id += 1;
  return {
    schedule_id: id, chore_id: id, title: `Chore ${id}`, description: '', category: 'core',
    points_value: 5, missed_penalty_value: 0, requires_approval: false, requires_photo: false,
    photo_source: 'child', auto_complete: false, assignment_type: 'individual', expiry_penalty: 'block',
    expiry_penalty_value: 0, available: true, expired: false, completed: false,
    date: localDateStr(new Date()),
    ...over,
  } as ScheduledChore;
}

describe('ChoreGroups header count', () => {
  beforeAll(async () => { await i18n.changeLanguage('en'); });
  afterEach(() => { cleanup(); });

  it("counts like the day progress: a sibling's waiting take isn't done, the kid's own waiting one is", () => {
    const today = localDateStr(new Date());
    const ctx = { now: new Date(), today, bonusOpen: false, requiredDone: true, t: i18n.t };
    const chores = [
      chore({ completed: true, completion_status: 'approved' }),
      chore({ completed: true, completion_status: 'pending' }),
      chore({ assignment_type: 'fcfs', completed_by_sibling: true, completed_by_name: 'Bob' }),
    ];
    render(<ChoreGroups chores={chores} viewOf={c => choreView(c, ctx)} />);
    expect(screen.getByText('2 of 3')).toBeTruthy();
  });
});
