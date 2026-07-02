class Widget:
    name: str

    def render(self) -> str:
        # leading comment must not leak into the signature
        html = build(self.name)
        return html
