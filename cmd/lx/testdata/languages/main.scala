/** Geometric shape abstraction. */
trait Shape {
    def area(): Double
    def perimeter(): Double
}

/**
 * Immutable 2-D point.
 *
 * @param x horizontal coordinate
 * @param y vertical coordinate
 */
case class Point(x: Double, y: Double) {
    val label: String = "" // display label
    private val secret: Int = 42 /* unused */

    // Scale uniformly.
    def scale(factor: Double): Point = Point(x * factor, y * factor)

    private def helper(): Unit = ()
}

// Singleton calculator.
object Calculator {
    val Max: Int = 100

    /** Add x to Max. */
    def add(x: Int): Int = x + Max
}

def topLevel(x: Int): Int = x + 1 // simple increment

/* Combine two values. */
def combine(
    x: Int,
    y: Int,
): Int = x + y
