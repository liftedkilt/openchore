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
  const digits = async (pin: string) => {
    for (const d of pin) await userEvent.click(screen.getByRole('button', { name: d }));
  };

  it('submits on the last digit when the length is known', async () => {
    const onSubmit = vi.fn();
    render(<PinPad prompt="Enter your PIN" length={4} onSubmit={onSubmit} />);
    await digits('1234');
    expect(onSubmit).toHaveBeenCalledWith('1234');
    expect(screen.queryByRole('button', { name: 'Done' })).toBeNull();
  });

  it('takes a 6-digit PIN when told its length', async () => {
    const onSubmit = vi.fn();
    render(<PinPad length={6} onSubmit={onSubmit} />);
    await digits('1234');
    expect(onSubmit).not.toHaveBeenCalled();
    await digits('567');
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith('123456');
  });

  it('deletes the last digit', async () => {
    const onSubmit = vi.fn();
    render(<PinPad length={4} onSubmit={onSubmit} />);
    const del = screen.getByRole('button', { name: 'Delete' });
    expect((del as HTMLButtonElement).disabled).toBe(true);
    await digits('12');
    await userEvent.click(del);
    await digits('345');
    expect(onSubmit).toHaveBeenCalledWith('1345');
  });

  it('without a length, takes 4-8 digits and submits with the check key', async () => {
    const onSubmit = vi.fn();
    render(<PinPad onSubmit={onSubmit} />);
    const done = screen.getByRole('button', { name: 'Done' }) as HTMLButtonElement;
    await digits('123');
    expect(done.disabled).toBe(true);
    await digits('4567');
    expect(onSubmit).not.toHaveBeenCalled();
    expect(done.disabled).toBe(false);
    await userEvent.click(done);
    await userEvent.click(done);
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith('1234567');
  });

  it('stops at 8 digits and submits with Enter', async () => {
    const onSubmit = vi.fn();
    render(<PinPad onSubmit={onSubmit} />);
    await userEvent.keyboard('1234567890{Enter}');
    expect(onSubmit).toHaveBeenCalledWith('12345678');
  });

  it('ignores Enter before the shortest PIN', async () => {
    const onSubmit = vi.fn();
    render(<PinPad onSubmit={onSubmit} />);
    await userEvent.keyboard('123{Enter}');
    expect(onSubmit).not.toHaveBeenCalled();
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
