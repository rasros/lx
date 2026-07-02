@dataclass
class Config:
    name: str

    @property
    def label(self) -> str:
        return self.name


@cache
def load() -> Config:
    return Config("x")
