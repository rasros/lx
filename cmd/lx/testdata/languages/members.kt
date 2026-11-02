package p

class K(val a: Int) {
    val prop: Int = 1

    constructor(s: String) : this(1) {
        println(s)
    }

    companion object {
        const val X = 1

        fun create(): K {
            return K(1)
        }
    }

    private fun hidden() {}
}

// An alias is a type.
typealias Dirs = List<Dir>

enum class Dir {
    N,
    S;

    fun flip(): Dir {
        return this
    }
}
