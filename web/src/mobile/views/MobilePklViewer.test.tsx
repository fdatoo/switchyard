import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MobilePklViewer } from "./MobilePklViewer";

// Monaco is heavy — mock it for unit tests
vi.mock("@monaco-editor/react", () => ({
  default: ({ value }: { value: string }) => <pre data-testid="monaco-mock">{value}</pre>,
}));

const SOURCE = `amends "package://pkg.pkl-lang.org/pkl-k8s/k8s@1.0.0#/Deployment.pkl"\nname = "switchyardd"`;

describe("MobilePklViewer", () => {
  it("renders source in Monaco (mocked)", () => {
    render(<MobilePklViewer source={SOURCE} path="switchyardd.pkl" />);
    expect(screen.getByTestId("monaco-mock")).toHaveTextContent("switchyardd");
  });

  it("shows read-only label", () => {
    render(<MobilePklViewer source={SOURCE} path="switchyardd.pkl" />);
    expect(screen.getByText(/read.only/i)).toBeInTheDocument();
  });
});
