sub greet {
    my ($name, $greeting) = @_;
    return "$greeting, $name";
}

sub add {
    my ($a, $b) = @_;
    return $a + $b;
}

sub _helper {
    return 0;
}
