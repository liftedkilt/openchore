// Pick who a gift or a playdate is for. A gift goes to one friend's animal;
// a playdate sends your own animal to a friend family's farm for the day.
import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { api } from '../../../api';
import type { FriendFarm, Pet, PetPick, Reward } from '../../../types';
import { Sheet } from '../Sheet';
import { FarmAnimal, petPicture } from './FarmAnimal';
import k from '../kid.module.css';
import s from './petSheet.module.css';

interface FriendPickSheetProps {
  pet: Pet;
  reward: Reward;
  busy?: boolean;
  onConfirm: (pick: PetPick) => void;
  onClose: () => void;
}

export function FriendPickSheet({ pet, reward, busy, onConfirm, onClose }: FriendPickSheetProps) {
  const { t } = useTranslation();
  const gift = reward.pet_action === 'gift';
  const [farms, setFarms] = useState<FriendFarm[] | null>(null);
  const [friend, setFriend] = useState('');
  const [link, setLink] = useState(0);

  useEffect(() => {
    api.friends.farms().then(r => setFarms(r.farms)).catch(() => setFarms([]));
  }, []);

  const chosen = gift ? !!friend : link > 0;
  return (
    <Sheet title={t(gift ? 'kid.farm.friend.giftTitle' : 'kid.farm.friend.playdateTitle')} onClose={onClose}>
      <p className={k.helpLine}>{t(gift ? 'kid.farm.friend.giftLead' : 'kid.farm.friend.playdateLead', { name: pet.name })}</p>
      {farms?.length === 0 && <p className={k.helpLine}>{t('kid.farm.friend.none')}</p>}

      {(farms ?? []).map(f => (
        <section key={f.link_id} className={s.section}>
          <h3 className={s.subheading}>{f.family}</h3>
          {gift ? (
            <div className={s.species} role="radiogroup" aria-label={f.family}>
              {f.pets.map(p => {
                const id = p.friend_key ?? '';
                return (
                  <button key={p.id} type="button" role="radio" aria-checked={friend === id}
                    className={clsx(s.speciesBtn, friend === id && s.speciesOn)} onClick={() => setFriend(id)}>
                    <FarmAnimal species={p.species} color={p.color} mood={p.mood} accessory={p.accessory} {...petPicture(p)} className={s.speciesArt} />
                    <span>{p.name}</span>
                    <span className={s.friendOwner}>{p.owner_name}</span>
                  </button>
                );
              })}
            </div>
          ) : (
            <button type="button" role="radio" aria-checked={link === f.link_id}
              className={clsx(s.chip, link === f.link_id && s.chipOn)} onClick={() => setLink(f.link_id)}>
              {t('kid.farm.friend.visit', { family: f.family })}
            </button>
          )}
        </section>
      ))}

      <div className={k.sheetActions}>
        <button type="button" className={k.primaryAction} disabled={!chosen || busy}
          onClick={() => onConfirm(gift ? { friend } : { link })}>
          {t('kid.farm.pick.confirm', { count: reward.effective_cost })}
        </button>
      </div>
    </Sheet>
  );
}
