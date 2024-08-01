package net.consensys

import java.math.BigDecimal
import java.math.BigInteger
import java.math.MathContext
import java.math.RoundingMode

const val OneGWei = 1_000_000_000L
val OneGWeiBigDecimal: BigDecimal = BigDecimal.valueOf(OneGWei)

const val OneKWei = 1_000L
val OneKWeiBigDecimal: BigDecimal = BigDecimal.valueOf(OneKWei)

inline val Long.gwei get() = Math.multiplyExact(this, OneGWei)

// Double extensions
fun Double.toGWei(): Double = this / OneGWei
fun Double.tokWeiUInt(): UInt = (this / OneKWei).toUInt()
fun Double.toKWei(): Double = this / OneKWei

// BigDecimal extensions
fun BigDecimal.roundUpToBigInteger(): BigInteger = this.setScale(0, RoundingMode.HALF_UP).toBigInteger()
fun BigDecimal.toGWei(): BigDecimal = this.divide(OneGWeiBigDecimal, MathContext.DECIMAL128)
fun BigDecimal.toKWei(): BigDecimal = this.divide(OneKWeiBigDecimal, MathContext.DECIMAL128)
fun BigDecimal.toUInt(): UInt = this.roundUpToBigInteger().toUInt()

// BigInteger extensions
fun BigInteger.multiply(multiplicand: Double): BigInteger = this.toBigDecimal()
  .multiply(BigDecimal.valueOf(multiplicand)).toBigInteger()
fun BigInteger.toGWei(): BigDecimal = this.toBigDecimal().toGWei()
inline val BigInteger.gwei: BigInteger get() = this.multiply(OneGWei.toBigInteger())
fun BigInteger.toKWei(): BigDecimal = this.toBigDecimal().toKWei()
inline val BigInteger.kwei: BigInteger get() = this.multiply(OneKWei.toBigInteger())
fun BigInteger.toULong(): ULong = this.toString().toULong()
fun BigInteger.toUInt(): UInt = this.toString().toUInt()

// ULong extensions
fun ULong.toBigInteger(): BigInteger = BigInteger(this.toString())
fun ULong.toHexString(): String = "0x${this.toString(16)}"

fun ULong.toKWeiUInt(): UInt = this.toDouble().tokWeiUInt()

inline val ULong.gwei: ULong get() = this * OneGWei.toULong()

fun ULong.toGWei(): Double = this.toDouble().toGWei()

/**
 * Parses an hexadecimal string as [ULong] number and returns the result.
 * @throws NumberFormatException if the string is not a valid hexadecimal representation of a number.
 */
fun ULong.Companion.fromHexString(value: String): ULong = value.replace("0x", "").toULong(16)

// ULong extensions
fun ByteArray.encodeHex(prefix: Boolean = true): String =
  "${if (prefix) "0x" else ""}${joinToString(separator = "") { eachByte -> "%02x".format(eachByte) }}"

fun String.decodeHex(): ByteArray {
  check(length % 2 == 0) { "Must have an even length" }
  return removePrefix("0x").chunked(2)
    .map { it.toInt(16).toByte() }
    .toByteArray()
}

fun String.containsAny(strings: List<String>, ignoreCase: Boolean): Boolean {
  return strings.any { this.contains(it, ignoreCase) }
}

fun <T : Comparable<T>> ClosedRange<T>.toIntervalString(): String {
  val size = if (start <= endInclusive) {
    this.endInclusive.toString().toBigDecimal() - this.start.toString().toBigDecimal() + 1.toBigDecimal()
  } else {
    this.start.toString().toBigDecimal() - this.endInclusive.toString().toBigDecimal() + 1.toBigDecimal()
  }
  return "[${this.start}..${this.endInclusive}]${size.toInt()}"
}

fun ByteArray.assertSize(expectedSize: UInt, fieldName: String = ""): ByteArray = apply {
  require(size == expectedSize.toInt()) { "$fieldName expected to have $expectedSize bytes, but got $size" }
}

fun ByteArray.assertIs32Bytes(fieldName: String = ""): ByteArray = assertSize(32u, fieldName)

fun ByteArray.assertIs20Bytes(fieldName: String = ""): ByteArray = assertSize(20u, fieldName)

fun ByteArray.setFirstByteToZero(): ByteArray {
  this[0] = 0
  return this
}