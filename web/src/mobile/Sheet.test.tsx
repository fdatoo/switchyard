import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { Sheet, SheetTrigger, SheetContent } from "./Sheet";

describe("Sheet", () => {
  it("Sheet is closed by default", () => {
    render(
      <Sheet>
        <SheetTrigger>Open</SheetTrigger>
        <SheetContent>Sheet body</SheetContent>
      </Sheet>,
    );
    expect(screen.queryByText("Sheet body")).not.toBeInTheDocument();
  });

  it("Sheet opens on trigger click", async () => {
    const user = userEvent.setup();
    render(
      <Sheet>
        <SheetTrigger>Open</SheetTrigger>
        <SheetContent>Sheet body</SheetContent>
      </Sheet>,
    );
    await user.click(screen.getByText("Open"));
    expect(screen.getByText("Sheet body")).toBeInTheDocument();
  });
});
