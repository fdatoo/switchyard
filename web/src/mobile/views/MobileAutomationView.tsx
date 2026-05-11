// TODO(plan-3): wire to AutomationService when available
import styles from "./MobileAutomationView.module.css";

interface Automation {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  lastRun: string | null;
}

interface Props {
  automation: Automation;
}

export function MobileAutomationView({ automation }: Props) {
  return (
    <div className={styles.page}>
      <div className={styles.banner} role="note">
        Full editing on a larger screen only. This is a read-only view.
      </div>
      <h1 className={styles.name}>{automation.name}</h1>
      <p className={styles.description}>{automation.description}</p>
      {automation.lastRun && (
        <p className={styles.meta}>Last run: {new Date(automation.lastRun).toLocaleString()}</p>
      )}
      <div className={styles.actions}>
        <button className={styles.btn}>Run</button>
        <button className={styles.btn}>View</button>
        <button className={styles.btn}>{automation.enabled ? "Disable" : "Enable"}</button>
      </div>
    </div>
  );
}
