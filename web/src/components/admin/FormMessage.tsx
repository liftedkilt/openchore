import React from 'react';
import { Icon } from '../../design';
import ui from './ui.module.css';

export type Msg = { type: 'success' | 'error'; text: string } | null;

/** A saved / failed line under a settings form. */
export const FormMessage: React.FC<{ msg: Msg }> = ({ msg }) => {
  if (!msg) return null;
  return msg.type === 'success'
    ? <p className={ui.msg} role="status"><Icon name="check" /> {msg.text}</p>
    : <p className={ui.msgError} role="alert">{msg.text}</p>;
};
