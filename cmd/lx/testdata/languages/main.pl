# Greet someone by name and greeting.
sub greet {
    my ($name, $greeting) = @_;
    return "$greeting, $name";
}

# Add two numbers.
sub add {
    my ($a, $b) = @_;
    return $a + $b;
}

sub _helper { # internal
    return 0;
}
