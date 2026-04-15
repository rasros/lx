import React from "react";

export interface Props {
    title: string;
}

export class Panel extends React.Component<Props> {
    public value: number;
    private secret: string;

    constructor(props: Props) {
        super(props);
        this.value = 1;
        this.secret = "";
    }

    public render(): JSX.Element {
        return <div>{this.props.title}</div>;
    }
}

export function buildPanel(title: string): JSX.Element {
    return <Panel title={title} />;
}

export function combine(
    title: string,
    value: number,
): JSX.Element {
    return <Panel title={title} />;
}
