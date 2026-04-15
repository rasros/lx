data class Point(val x: Double, val y: Double)

interface Shape {
    fun area(): Double
    fun perimeter(): Double
}

class Calculator {
    val max: Int = 100
    private val secret: Int = 42

    fun add(x: Int): Int {
        return x + max
    }

    private fun helper(): Int = 42
}

fun topLevel(x: Int): Int {
    return x + 1
}
