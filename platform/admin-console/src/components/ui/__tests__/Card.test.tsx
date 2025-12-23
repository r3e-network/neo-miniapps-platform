// =============================================================================
// Tests for Card Component
// =============================================================================

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Card, CardHeader, CardTitle, CardContent, CardFooter } from "@/components/ui/Card";

describe("Card Component", () => {
  it("should render card with content", () => {
    render(
      <Card>
        <CardContent>Test content</CardContent>
      </Card>,
    );
    expect(screen.getByText("Test content")).toBeInTheDocument();
  });

  it("should apply default variant", () => {
    const { container } = render(<Card>Content</Card>);
    const card = container.firstChild as HTMLElement;
    expect(card).toHaveClass("bg-white", "shadow-sm");
  });

  it("should apply bordered variant", () => {
    const { container } = render(<Card variant="bordered">Content</Card>);
    const card = container.firstChild as HTMLElement;
    expect(card).toHaveClass("border", "border-gray-200");
  });

  it("should render card with header", () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>Test Title</CardTitle>
        </CardHeader>
      </Card>,
    );
    expect(screen.getByText("Test Title")).toBeInTheDocument();
  });

  it("should render card with footer", () => {
    render(
      <Card>
        <CardFooter>Footer content</CardFooter>
      </Card>,
    );
    expect(screen.getByText("Footer content")).toBeInTheDocument();
  });

  it("should render complete card structure", () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>Title</CardTitle>
        </CardHeader>
        <CardContent>Content</CardContent>
        <CardFooter>Footer</CardFooter>
      </Card>,
    );
    expect(screen.getByText("Title")).toBeInTheDocument();
    expect(screen.getByText("Content")).toBeInTheDocument();
    expect(screen.getByText("Footer")).toBeInTheDocument();
  });
});
