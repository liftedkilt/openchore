// @vitest-environment jsdom
import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../../../i18n';
import type { Pet, Reward } from '../../../types';
import { PictureSheet } from './PictureSheet';
import { STARTER_PACK } from './starterPack';

// Minimal valid 1x1 transparent PNG bytes.
const TINY_PNG_BYTES = new Uint8Array([
  137, 80, 78, 71, 13, 10, 26, 10,
  0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 31, 21, 196, 137,
  0, 0, 0, 10, 73, 68, 65, 84, 120, 156, 99, 0, 1, 0, 0, 5, 0, 1, 13, 10, 45, 180,
  0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130,
]);

const PET: Pet = {
  id: 1,
  user_id: 3,
  name: 'Quackers',
  species: 'chicken',
  color: 'white',
  health: 100,
  fullness: 100,
  joy: 100,
  accessory: 'straw_hat',
  items: ['straw_hat'],
  mood: 'content',
  hiding: false,
  sick: false,
  can_play: true,
};

const REWARD: Reward = {
  id: 9,
  name: 'Use my own picture',
  description: '',
  cost: 60,
  effective_cost: 60,
  active: true,
  shareable: false,
  pet_action: 'picture',
  created_by: 1,
  created_at: '2026-01-01T00:00:00Z',
};

function makePngResponse(bytes: Uint8Array = TINY_PNG_BYTES, mime = 'image/png'): Response {
  const copy = new Uint8Array(bytes);
  return {
    ok: true,
    status: 200,
    blob: async () => new Blob([copy], { type: mime }),
  } as unknown as Response;
}

beforeAll(async () => {
  if (!i18n.isInitialized) await i18n.init();
  await i18n.changeLanguage('en');
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('PictureSheet', () => {
  it('renders all 8 starter-pack radios unchecked with pixelated thumbnails and disabled confirm', () => {
    render(<PictureSheet pet={PET} reward={REWARD} onConfirm={vi.fn()} onClose={vi.fn()} />);

    const group = screen.getByRole('radiogroup', { name: 'Ready-made animals' });
    const radios = screen.getAllByRole('radio');
    expect(radios).toHaveLength(STARTER_PACK.length);

    for (const animal of STARTER_PACK) {
      const radio = screen.getByRole('radio', { name: i18n.t(animal.labelKey) });
      expect(radio.getAttribute('aria-checked')).toBe('false');
      const img = radio.querySelector('image');
      expect(img?.getAttribute('href')).toBe(animal.stripUrl);
    }
    expect(group).toBeTruthy();

    const confirm = screen.getByRole('button', { name: /Get it for 60/ }) as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);
  });

  it('selects a starter-pack animal, shows 6-frame mood previews, and submits a PNG data URL', async () => {
    const fetchMock = vi.fn().mockResolvedValue(makePngResponse());
    vi.stubGlobal('fetch', fetchMock);
    const onConfirm = vi.fn();

    render(<PictureSheet pet={PET} reward={REWARD} onConfirm={onConfirm} onClose={vi.fn()} />);

    const donkey = screen.getByRole('radio', { name: 'Donkey' });
    await userEvent.click(donkey);

    await waitFor(() => {
      expect(donkey.getAttribute('aria-checked')).toBe('true');
    });
    expect(fetchMock).toHaveBeenCalledWith('/sprites/pack/donkey-strip.png');
    expect(screen.getByText('6 pictures found in a row.')).toBeTruthy();
    expect(screen.getByText('Sick')).toBeTruthy();
    expect(screen.getByText('Very sick')).toBeTruthy();
    expect(screen.getByText('Happy')).toBeTruthy();

    const confirm = screen.getByRole('button', { name: /Get it for 60/ }) as HTMLButtonElement;
    expect(confirm.disabled).toBe(false);
    await userEvent.click(confirm);

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const arg = onConfirm.mock.calls[0][0] as { image: string };
    expect(arg.image).toMatch(/^data:image\/png;base64,[A-Za-z0-9+/=]+$/);
  });

  it('ignores stale responses when tapping starter-pack animals rapidly', async () => {
    let resolveDonkey!: (resp: Response) => void;
    const donkeyPromise = new Promise<Response>((resolve) => { resolveDonkey = resolve; });
    const duckBytes = new Uint8Array([...TINY_PNG_BYTES, 1, 2, 3]);
    const fetchMock = vi.fn((url: string) => {
      if (url.includes('donkey')) return donkeyPromise;
      return Promise.resolve(makePngResponse(duckBytes));
    });
    vi.stubGlobal('fetch', fetchMock);
    const onConfirm = vi.fn();

    render(<PictureSheet pet={PET} reward={REWARD} onConfirm={onConfirm} onClose={vi.fn()} />);

    const donkey = screen.getByRole('radio', { name: 'Donkey' });
    const duck = screen.getByRole('radio', { name: 'Duck' });

    await userEvent.click(donkey);
    await userEvent.click(duck);

    await waitFor(() => {
      expect(duck.getAttribute('aria-checked')).toBe('true');
    });

    // Resolve Donkey late: Duck must stay selected and Donkey must not overwrite it.
    resolveDonkey(makePngResponse(TINY_PNG_BYTES));
    await new Promise(resolve => setTimeout(resolve, 25));

    expect(duck.getAttribute('aria-checked')).toBe('true');
    expect(donkey.getAttribute('aria-checked')).toBe('false');

    await userEvent.click(screen.getByRole('button', { name: /Get it for 60/ }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    const submitted = (onConfirm.mock.calls[0][0] as { image: string }).image;
    const expectedDuckBase64 = btoa(String.fromCharCode(...duckBytes));
    expect(submitted).toBe(`data:image/png;base64,${expectedDuckBase64}`);
  });

  it('coerces blobs with an empty MIME type into data:image/png;base64,...', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(makePngResponse(TINY_PNG_BYTES, '')));
    const onConfirm = vi.fn();

    render(<PictureSheet pet={PET} reward={REWARD} onConfirm={onConfirm} onClose={vi.fn()} />);

    await userEvent.click(screen.getByRole('radio', { name: 'Alpaca' }));
    const confirm = screen.getByRole('button', { name: /Get it for 60/ }) as HTMLButtonElement;
    await waitFor(() => expect(confirm.disabled).toBe(false));

    await userEvent.click(confirm);
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect((onConfirm.mock.calls[0][0] as { image: string }).image).toMatch(/^data:image\/png;base64,[A-Za-z0-9+/=]+$/);
  });

  it('shows an alert and keeps confirm disabled when fetching a starter-pack animal fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500 } as Response));

    render(<PictureSheet pet={PET} reward={REWARD} onConfirm={vi.fn()} onClose={vi.fn()} />);

    await userEvent.click(screen.getByRole('radio', { name: 'Rooster' }));

    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toContain("Couldn't load that animal picture");
    const confirm = screen.getByRole('button', { name: /Get it for 60/ }) as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);
  });

  it('clears starter-pack selection when a custom file is uploaded', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(makePngResponse()));

    // Mock Image probe so jsdom fires onload with 64x64 dimensions for the uploaded file.
    class FakeImage {
      naturalWidth = 64;
      naturalHeight = 64;
      onload: (() => void) | null = null;
      onerror: (() => void) | null = null;
      set src(_val: string) {
        queueMicrotask(() => this.onload?.());
      }
    }
    vi.stubGlobal('Image', FakeImage);

    const onConfirm = vi.fn();
    const { container } = render(<PictureSheet pet={PET} reward={REWARD} onConfirm={onConfirm} onClose={vi.fn()} />);

    const donkey = screen.getByRole('radio', { name: 'Donkey' });
    await userEvent.click(donkey);
    await waitFor(() => expect(donkey.getAttribute('aria-checked')).toBe('true'));

    const input = container.querySelector('input[type="file"]') as HTMLInputElement;

    // First try a non-PNG: clears Donkey and shows alert.
    const badFile = new File(['hello'], 'bad.jpg', { type: 'image/jpeg' });
    fireEvent.change(input, { target: { files: [badFile] } });
    expect(donkey.getAttribute('aria-checked')).toBe('false');
    expect(screen.getByRole('alert').textContent).toContain("isn't a PNG");

    // Re-select Donkey, then upload a valid single-frame PNG.
    await userEvent.click(donkey);
    await waitFor(() => expect(donkey.getAttribute('aria-checked')).toBe('true'));

    const goodFile = new File([TINY_PNG_BYTES], 'custom.png', { type: 'image/png' });
    fireEvent.change(input, { target: { files: [goodFile] } });

    await waitFor(() => {
      expect(screen.getByText('One picture. It will bob when it walks.')).toBeTruthy();
    });
    expect(donkey.getAttribute('aria-checked')).toBe('false');

    await userEvent.click(screen.getByRole('button', { name: /Get it for 60/ }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect((onConfirm.mock.calls[0][0] as { image: string }).image).toMatch(/^data:image\/png;base64,/);
  });

  it('disables starter-pack radios and confirm button while busy', () => {
    render(<PictureSheet pet={PET} reward={REWARD} busy onConfirm={vi.fn()} onClose={vi.fn()} />);

    for (const radio of screen.getAllByRole('radio') as HTMLButtonElement[]) {
      expect(radio.disabled).toBe(true);
    }
    const confirm = screen.getByRole('button', { name: /Get it for 60/ }) as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);
  });
});
