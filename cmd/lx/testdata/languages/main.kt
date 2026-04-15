/** Immutable 2-D point. */
data class Point(val x: Double, val y: Double)

// Shape abstraction.
interface Shape {
    fun area(): Double
    fun perimeter(): Double
}

/**
 * Calculator with a configurable maximum.
 */
class Calculator {
    val max: Int = 100 // upper bound
    private val secret: Int = 42 /* hidden */

    // Add x to the max.
    fun add(x: Int): Int {
        return x + max
    }

    private fun helper(): Int = 42
}

/** Simple top-level increment. */
fun topLevel(x: Int): Int {
    return x + 1
}

// Combine two integers.
fun combine(
    x: Int,
    y: Int,
): Int {
    return x + y
}
