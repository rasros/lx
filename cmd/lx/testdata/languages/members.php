<?php
/** Greets things. */
trait Greets {
    public $greeting = 'hi';

    public function greet(): string {
        return $this->greeting;
    }

    private function secret() {}
}

enum Suit: string {
    case Hearts = 'H';
    case Spades = 'S';

    public function label(): string {
        return ucfirst($this->value);
    }
}
