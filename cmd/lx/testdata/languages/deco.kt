@Serializable
class User(val name: String) {
    @Deprecated("use name")
    fun legacy(): String {
        return name
    }
}
