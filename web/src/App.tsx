import { useTheme } from "./theme/provider";

export default function App() {
  const { mode } = useTheme();
  return (
    <div>
      <p>gohome — theme: {mode}</p>
    </div>
  );
}
