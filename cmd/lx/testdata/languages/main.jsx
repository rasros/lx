import React from "react";

export class Card extends React.Component {
    title = "default";

    render() {
        return <div>{this.title}</div>;
    }
}

export function makeCard(name) {
    return <Card title={name} />;
}

export function buildCard(
    name,
    title,
) {
    return <Card title={title} />;
}
