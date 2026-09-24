// @vitest-environment jsdom
import { describe, it, expect, beforeAll, vi, afterEach } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../../i18n';
import { PinPad } from './PinPad';

beforeAll(async () => {
  if (!i18n.isInitialized) await i18n.init();
  await i18n.changeLanguage('en');
});

afterEach(cleanup);

describe('PinPad', () => {
  it('submits once the PIN is complete', async () => {
    const onSubmit = vi.fn();
    render(<PinPad prompt="Enter your PIN" onSubmit={onSubmit} />);
    for (const d of '1234') await userEvent.click(screen.getByRole('button', { name: d }));
    expect(onSubmit).toHaveBeenCalledWith('1234');
  });

  it('deletes the last digit', async () => {
    const onSubmit = vi.fn();
    render(<PinPad onSubmit={onSubmit} />);
    const del = screen.getByRole('button', { name: 'Delete' });
    expect((del as HTMLButtonElement).disabled).toBe(true);
    for (const d of '12') await userEvent.click(screen.getByRole('button', { name: d }));
    await userEvent.click(del);
    for (const d of '345') await userEvent.click(screen.getByRole('button', { name: d }));
    expect(onSubmit).toHaveBeenCalledWith('1345');
  });

  it('states an error in words, as an alert', () => {
    render(<PinPad onSubmit={() => {}} error="Incorrect PIN" />);
    expect(screen.getByRole('alert').textContent).toBe('Incorrect PIN');
  });

  it('labels the keypad with the prompt', () => {
    render(<PinPad prompt="Enter your PIN" onSubmit={() => {}} />);
    expect(screen.getByRole('group', { name: 'Enter your PIN' })).toBeDefined();
  });
});
