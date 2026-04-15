class Animal:
    species = "unknown"
    _tag = "internal"

    def speak(self):
        return "..."

    def _private(self):
        return "x"


def top_level():
    return 1

def _secret():
    return 0
