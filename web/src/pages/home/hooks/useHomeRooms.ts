// TODO(plan-02-rooms): replace with EntityService.Subscribe room entities

export interface RoomSummary {
  id: string;
  name: string;
  entityCount: string;
  scenes: string[];
  statePill: string;
}

/**
 * useHomeRooms — returns up to 8 room summaries for the Rooms grid.
 * Mock data; Plan 02-rooms will wire this to live room entity subscriptions.
 */
export function useHomeRooms(): RoomSummary[] {
  return [
    {
      id: "kitchen",
      name: "Kitchen",
      entityCount: "3 lights",
      scenes: ["Morning", "Cooking", "Evening"],
      statePill: "Bright",
    },
    {
      id: "living-room",
      name: "Living Room",
      entityCount: "5 lights",
      scenes: ["Movie", "Reading", "Party"],
      statePill: "Bright",
    },
    {
      id: "office",
      name: "Office",
      entityCount: "2 lights",
      scenes: ["Focus", "Relax"],
      statePill: "On",
    },
    {
      id: "bedroom",
      name: "Bedroom",
      entityCount: "2 lights",
      scenes: ["Sleep", "Wake"],
      statePill: "Off",
    },
    {
      id: "bathroom",
      name: "Bathroom",
      entityCount: "1 light",
      scenes: ["Morning", "Night"],
      statePill: "Off",
    },
    {
      id: "hallway",
      name: "Hallway",
      entityCount: "2 lights",
      scenes: ["Dim"],
      statePill: "Dim",
    },
  ];
}
