import { describe, it, expect, vi, afterEach } from "vitest";
import { useHomeGreeting } from "./useHomeGreeting";

// Helper to mock Date to a specific hour/minute
function mockHour(hour: number, minute = 0) {
  const now = new Date(2026, 0, 15, hour, minute, 0);
  vi.setSystemTime(now);
}

afterEach(() => {
  vi.useRealTimers();
});

describe("useHomeGreeting", () => {
  it('returns "Good morning" between 05:00 and 11:59', () => {
    vi.useFakeTimers();
    mockHour(8, 30);
    const { greeting } = useHomeGreeting({ alertCount: 0 });
    expect(greeting).toBe("Good morning");
  });

  it('returns "Good afternoon" at noon', () => {
    vi.useFakeTimers();
    mockHour(12, 0);
    const { greeting } = useHomeGreeting({ alertCount: 0 });
    expect(greeting).toBe("Good afternoon");
  });

  it('returns "Good evening" at 17:30', () => {
    vi.useFakeTimers();
    mockHour(17, 30);
    const { greeting } = useHomeGreeting({ alertCount: 0 });
    expect(greeting).toBe("Good evening");
  });

  it('returns "Good night" at 21:00', () => {
    vi.useFakeTimers();
    mockHour(21, 0);
    const { greeting } = useHomeGreeting({ alertCount: 0 });
    expect(greeting).toBe("Good night");
  });

  it("returns calm copy when alertCount is 0", () => {
    vi.useFakeTimers();
    mockHour(10, 0);
    const { statusLine } = useHomeGreeting({ alertCount: 0 });
    expect(statusLine).toBe("· everything looks calm.");
  });

  it("returns singular alert copy when alertCount is 1", () => {
    vi.useFakeTimers();
    mockHour(10, 0);
    const { statusLine } = useHomeGreeting({ alertCount: 1 });
    expect(statusLine).toBe("· 1 thing needs attention.");
  });

  it("returns plural copy when alertCount is 3", () => {
    vi.useFakeTimers();
    mockHour(10, 0);
    const { statusLine } = useHomeGreeting({ alertCount: 3 });
    expect(statusLine).toBe("· 3 things need attention.");
  });
});
