trait Shape {
    def area(): Double
    def perimeter(): Double
}

case class Point(x: Double, y: Double) {
    val label: String = ""
    private val secret: Int = 42

    def scale(factor: Double): Point = Point(x * factor, y * factor)

    private def helper(): Unit = ()
}

object Calculator {
    val Max: Int = 100

    def add(x: Int): Int = x + Max
}

def topLevel(x: Int): Int = x + 1
