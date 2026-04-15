import React from "react";

/** Props accepted by Panel. */
export interface Props {
    title: string; // display title
}

/**
 * Panel component with a value.
 */
export class Panel extends React.Component<Props> {
    public value: number;
    private secret: string; /* internal */

    constructor(props: Props) {
        super(props);
        this.value = 1;
        this.secret = "";
    }

    // Render the panel.
    public render(): JSX.Element {
        return <div>{this.props.title}</div>;
    }
}

/** Build a panel from a title. */
export function buildPanel(title: string): JSX.Element {
    return <Panel title={title} />;
}

// Combine title and value into a panel.
export function combine(
    title: string,
    value: number,
): JSX.Element {
    return <Panel title={title} />;
}
