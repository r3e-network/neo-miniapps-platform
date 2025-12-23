// =============================================================================
// Spinner Component Tests
// =============================================================================

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Spinner } from "../Spinner";

describe("Spinner Component", () => {
  it("should render spinner", () => {
    render(<Spinner data-testid="spinner" />);
    expect(screen.getByTestId("spinner")).toBeInTheDocument();
  });

  it("should apply small size to svg", () => {
    render(<Spinner size="sm" data-testid="small-spinner" />);
    const svg = screen.getByLabelText("Loading");
    expect(svg).toHaveClass("h-4");
    expect(svg).toHaveClass("w-4");
  });

  it("should apply medium size by default to svg", () => {
    render(<Spinner data-testid="medium-spinner" />);
    const svg = screen.getByLabelText("Loading");
    expect(svg).toHaveClass("h-8");
    expect(svg).toHaveClass("w-8");
  });

  it("should apply large size to svg", () => {
    render(<Spinner size="lg" data-testid="large-spinner" />);
    const svg = screen.getByLabelText("Loading");
    expect(svg).toHaveClass("h-12");
    expect(svg).toHaveClass("w-12");
  });

  it("should have animation class on svg", () => {
    render(<Spinner data-testid="animated-spinner" />);
    const svg = screen.getByLabelText("Loading");
    expect(svg).toHaveClass("animate-spin");
  });

  it("should apply custom className to container", () => {
    render(<Spinner className="custom-spinner" data-testid="custom" />);
    expect(screen.getByTestId("custom")).toHaveClass("custom-spinner");
  });

  it("should have accessible aria-label on svg", () => {
    render(<Spinner />);
    expect(screen.getByLabelText("Loading")).toBeInTheDocument();
  });

  it("should render svg element", () => {
    render(<Spinner />);
    const svg = screen.getByLabelText("Loading");
    expect(svg.tagName.toLowerCase()).toBe("svg");
  });
});
