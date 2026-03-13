import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';
import { Lock, Delete, ArrowLeft } from 'lucide-react';
import styles from './AdminPasscode.module.css';

export const AdminPasscode: React.FC = () => {
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [shaking, setShaking] = useState(false);
  const navigate = useNavigate();

  const handleDigit = (digit: string) => {
    if (code.length >= 6) return;
    const newCode = code + digit;
    setCode(newCode);
    setError('');

    if (newCode.length >= 4) {
      verify(newCode);
    }
  };

  const handleDelete = () => {
    setCode(prev => prev.slice(0, -1));
    setError('');
  };

  const verify = async (passcode: string) => {
    try {
      await api.admin.verifyPasscode(passcode);
      sessionStorage.setItem('openchore_admin', 'true');
      navigate('/admin/dashboard');
    } catch {
      setError('Incorrect passcode');
      setShaking(true);
      setTimeout(() => { setShaking(false); setCode(''); }, 600);
    }
  };

  const digits = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', 'del'];

  return (
    <div className={styles.container}>
      <button className={styles.backBtn} onClick={() => navigate('/login')}>
        <ArrowLeft size={20} /> Back
      </button>

      <div className={styles.content}>
        <div className={styles.iconWrapper}>
          <Lock size={32} />
        </div>
        <h1 className={styles.title}>Parent Access</h1>
        <p className={styles.subtitle}>Enter your passcode</p>

        <div className={`${styles.dots} ${shaking ? styles.shake : ''}`}>
          {[0, 1, 2, 3].map(i => (
            <div
              key={i}
              className={`${styles.dot} ${i < code.length ? styles.dotFilled : ''} ${error ? styles.dotError : ''}`}
            />
          ))}
        </div>

        {error && <p className={styles.error}>{error}</p>}

        <div className={styles.keypad}>
          {digits.map((d, i) => {
            if (d === '') return <div key={i} className={styles.keyEmpty} />;
            if (d === 'del') {
              return (
                <button key={i} className={styles.key} onClick={handleDelete}>
                  <Delete size={22} />
                </button>
              );
            }
            return (
              <button key={i} className={styles.key} onClick={() => handleDigit(d)}>
                {d}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
};
