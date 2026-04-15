import React from "react";

/* Card component with a title. */
export class Card extends React.Component {
    title = "default"; // fallback title

    // Render the card.
    render() {
        return <div>{this.title}</div>;
    }
}

/** Create a card element. */
export function makeCard(name) {
    return <Card title={name} />;
}

// Build a card with explicit name and title.
export function buildCard(
    name,
    title,
) {
    return <Card title={title} />;
}
