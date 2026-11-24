// @vitest-environment jsdom
import { describe, it, expect, beforeAll, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../../i18n';
import { api } from '../../api';
import type { FriendLink, FriendsSettings } from '../../types';
import { FriendFamiliesSection } from './FriendFamiliesSection';

vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      friends: {
        ...actual.api.friends,
        settings: vi.fn(),
        update: vi.fn(),
        updateLink: vi.fn(),
        invite: vi.fn(),
        join: vi.fn(),
        confirm: vi.fn(),
        sync: vi.fn(),
        unlink: vi.fn(),
      },
    },
  };
});

const baseLink = (overrides: Partial<FriendLink> = {}): FriendLink => ({
  id: 1,
  status: 'active',
  side: 'a',
  peer_pending: false,
  peer_name: 'The Smiths',
  hub_url: 'https://hub.example',
  show_pictures: false,
  allow_visits: true,
  goal: null,
  outbound: [{ id: 2, name: 'Ava', role: 'child', shared: false, accepted: false }],
  inbound: [{ id: '10', name: 'Bo', accepted: false }],
  snapshot_at: '2026-11-24T12:00:00Z',
  last_sync_at: '2026-11-24T12:00:00Z',
  last_error: '',
  created_at: '2026-11-24T10:00:00Z',
  ...overrides,
});

const baseSettings = (links: FriendLink[]): FriendsSettings => ({
  leaderboard_enabled: true,
  family_name: 'The Avas',
  hub_url: 'https://hub.example',
  hub_key_set: true,
  hub_from_env: false,
  links,
});

beforeAll(async () => {
  if (!i18n.isInitialized) await i18n.init();
  await i18n.changeLanguage('en');
});

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(cleanup);

describe('FriendFamiliesSection', () => {
  it('renders a waiting invite with its code and expiry text', async () => {
    vi.mocked(api.friends.settings).mockResolvedValue(
      baseSettings([
        baseLink({
          id: 5,
          status: 'waiting',
          peer_name: '',
          invite: 'ocf1.testinvitecode',
          invite_expires: '2026-11-25T12:00:00Z',
          inbound: [],
          snapshot_at: undefined,
        }),
      ]),
    );

    render(<FriendFamiliesSection />);

    expect(await screen.findByText('New invite')).toBeDefined();
    expect(screen.getByText('Waiting for them')).toBeDefined();
    expect((screen.getByLabelText('Invite') as HTMLInputElement).value).toBe('ocf1.testinvitecode');
    expect(screen.getByText(/It works once and runs out/)).toBeDefined();
    expect(screen.queryByRole('button', { name: 'Confirm' })).toBeNull();
  });

  it('renders pending_confirm prompt, hides sharing controls, and confirms or declines', async () => {
    const pending = baseLink({
      id: 7,
      status: 'pending_confirm',
      peer_name: 'The Garcias',
      inbound: [],
      snapshot_at: undefined,
    });
    vi.mocked(api.friends.settings).mockResolvedValue(baseSettings([pending]));
    vi.mocked(api.friends.confirm).mockResolvedValue(baseLink({ id: 7, status: 'active', peer_name: 'The Garcias' }));
    vi.mocked(api.friends.unlink).mockResolvedValue(undefined);

    render(<FriendFamiliesSection />);

    expect(await screen.findByText('The Garcias')).toBeDefined();
    expect(screen.getByText('Needs confirmation')).toBeDefined();
    expect(screen.getByText(/The Garcias used your invite/)).toBeDefined();

    // Sharing, pictures, visits, team goal, and duplicate Remove button are hidden until confirmed.
    expect(screen.queryByText(/Sent to The Garcias/)).toBeNull();
    expect(screen.queryByText(/Offered by The Garcias/)).toBeNull();
    expect(screen.queryByText(/Show custom animal pictures/)).toBeNull();
    expect(screen.queryByText(/Take gifts and playdates/)).toBeNull();
    expect(screen.queryByText('Team goal for this week')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Remove' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Update now' })).toBeDefined();

    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }));
    await waitFor(() => expect(api.friends.confirm).toHaveBeenCalledWith(7));

    await userEvent.click(screen.getByRole('button', { name: 'Decline' }));
    await waitFor(() => expect(api.friends.unlink).toHaveBeenCalledWith(7));
  });

  it('falls back to "Friend family" when pending_confirm has an empty peer_name', async () => {
    vi.mocked(api.friends.settings).mockResolvedValue(
      baseSettings([
        baseLink({
          id: 8,
          status: 'pending_confirm',
          peer_name: '',
          inbound: [],
          snapshot_at: undefined,
        }),
      ]),
    );

    render(<FriendFamiliesSection />);

    expect(await screen.findByText('Friend family')).toBeDefined();
    expect(screen.getByText(/Friend family used your invite/)).toBeDefined();
  });

  it('shows calm waiting-for-confirmation note on side b while peer_pending without an error banner', async () => {
    vi.mocked(api.friends.settings).mockResolvedValue(
      baseSettings([
        baseLink({
          id: 9,
          status: 'active',
          side: 'b',
          peer_pending: true,
          peer_name: 'The Avas',
          inbound: [],
          last_error: '',
        }),
      ]),
    );

    render(<FriendFamiliesSection />);

    expect(await screen.findByText('The Avas')).toBeDefined();
    expect(screen.getByText('Waiting for the other parent to confirm this link before updates appear.')).toBeDefined();
    expect(screen.queryByText(/Last update failed/)).toBeNull();
  });

  it('renders sharing and goal controls on an active confirmed link', async () => {
    vi.mocked(api.friends.settings).mockResolvedValue(baseSettings([baseLink()]));

    render(<FriendFamiliesSection />);

    expect(await screen.findByText('The Smiths')).toBeDefined();
    expect(screen.getByText('Linked')).toBeDefined();
    expect(screen.getByText('Sent to The Smiths')).toBeDefined();
    expect(screen.getByText('Offered by The Smiths')).toBeDefined();
    expect(screen.getByText('Team goal for this week')).toBeDefined();
    expect(screen.queryByText('Waiting for the other parent to confirm this link before updates appear.')).toBeNull();
  });
});
