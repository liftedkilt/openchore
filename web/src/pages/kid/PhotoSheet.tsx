import React, { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { QRCodeSVG } from 'qrcode.react';
import { api } from '../../api';
import type { ScheduledChore } from '../../types';
import { Icon } from '../../design';
import { Sheet } from './Sheet';
import s from './kid.module.css';

interface PhotoSheetProps {
  chore: ScheduledChore;
  userId: number;
  /** The household's public base URL (admin setting), for the QR link. */
  baseUrl?: string;
  onClose: () => void;
  /** The photo arrived and the chore is finished (or the proof attached). */
  onComplete: () => void;
}

/**
 * Photo proof for a chore: scan a QR code with another device, or take the
 * photo on this one. Polls until the photo (and completion) lands.
 */
export function PhotoSheet({ chore, userId, baseUrl, onClose, onComplete }: PhotoSheetProps) {
  const { t } = useTranslation();
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  // The phone that scans the code isn't signed in; the link carries a
  // short-lived token that can only upload a photo and complete this chore.
  const [uploadToken, setUploadToken] = useState<string | null>(null);
  const alreadyCompleted = chore.completed;
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    api.auth.uploadLink(chore.schedule_id)
      .then(r => setUploadToken(r.token))
      .catch(() => setUploadToken(null));
  }, [chore.schedule_id]);

  // Poll for the photo (or the completion) arriving from the other device.
  useEffect(() => {
    const interval = setInterval(async () => {
      try {
        const chores = await api.users.getChores(userId, 'daily', chore.date);
        const updated = chores.find(c => c.schedule_id === chore.schedule_id);
        if (alreadyCompleted ? updated?.photo_url : updated?.completed) onComplete();
      } catch (e) {
        console.error('Polling failed:', e);
      }
    }, 3000);
    return () => clearInterval(interval);
  }, [chore, userId, alreadyCompleted, onComplete]);

  const origin = baseUrl || window.location.origin;
  const uploadUrl = `${origin}/upload?scheduleId=${chore.schedule_id}&date=${chore.date}&userId=${userId}` +
    (uploadToken ? `&t=${encodeURIComponent(uploadToken)}` : '');

  const handleDirectUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    setUploadError(null);
    try {
      const { url } = await api.chores.upload(file);
      if (alreadyCompleted && chore.completion_id) {
        await api.chores.attachPhoto(chore.completion_id, url);
      } else {
        await api.chores.complete(chore.schedule_id, chore.date, url);
      }
      onComplete();
    } catch (err) {
      setUploadError((err as Error)?.message || t('kid.photo.uploadFailed'));
    } finally {
      setUploading(false);
    }
  };

  const handleCopyLink = async () => {
    try {
      await navigator.clipboard.writeText(uploadUrl);
    } catch {
      // Older browsers: copy through a temporary input.
      const input = document.createElement('input');
      input.value = uploadUrl;
      document.body.appendChild(input);
      input.select();
      document.execCommand('copy');
      document.body.removeChild(input);
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Sheet title={alreadyCompleted ? t('kid.photo.titleAdd') : t('kid.photo.titleNeeded')} onClose={onClose}>
      <p className={s.sheetLead}>
        <strong>{chore.title}</strong>
        <span>{t('kid.photo.scanOrUpload')}</span>
      </p>

      {/* The code sits on a House "paper" in every skin: scanners need dark on light. */}
      <div className={s.qrPaper} data-theme="house">
        {uploadToken
          ? <QRCodeSVG value={uploadUrl} size={224} marginSize={2} bgColor="transparent" fgColor="currentColor" title={t('kid.photo.qrLabel')} />
          : <span className={s.spinner} role="status" aria-label={t('kid.photo.preparing')} />}
      </div>

      <div className={s.sheetActions}>
        <button type="button" className={s.primaryAction} onClick={() => fileRef.current?.click()} disabled={uploading}>
          <Icon name="camera" />
          {uploading ? t('kid.photo.uploading') : t('kid.photo.takePhoto')}
        </button>
        <input ref={fileRef} type="file" accept="image/*" capture="environment" onChange={handleDirectUpload} hidden />
        <button type="button" className={s.secondaryAction} onClick={handleCopyLink}>
          {copied ? t('kid.photo.copied') : t('kid.photo.copyLink')}
        </button>
      </div>

      {uploadError && <p className={s.errorLine} role="alert">{uploadError}</p>}

      <p className={s.waitingLine} role="status">
        <span className={s.spinner} aria-hidden />
        {t('kid.photo.waiting')}
      </p>
      <p className={s.helpLine}>{alreadyCompleted ? t('kid.photo.helpAdd') : t('kid.photo.helpComplete')}</p>
    </Sheet>
  );
}
