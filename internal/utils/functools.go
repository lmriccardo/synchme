/**
 * Resembles the python functools module
 */

package utils

// Map applies the given function `fn` to each element of the input slice `in`,
// returning a new slice of results.
//
// It is a generic equivalent of the functional "map" operation found in many
// languages. The order of elements in the result matches the input.
//
// Type Parameters:
//   - T: the type of elements in the input slice
//   - U: the type of elements in the output slice
func Map[T, U any](fn func(T) U, in []T) []U {
	out := make([]U, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

// MapWithIndex applies the function `fn` to each element of the input slice `in`,
// passing both the element's index and value, and returns a new slice of results.
//
// This is similar to Python's map combined with enumerate(), allowing the mapping
// function to access the index as well as the value.
//
// Type Parameters:
//   - T: the type of elements in the input slice
//   - U: the type of elements in the output slice
func MapWithIndex[T, U any](fn func(int, T) U, in []T) []U {
	indexed := Enumerate(in)
	return Map(func(v Tuple[int, T]) U { return fn(v.First, v.Second) }, indexed)
}

// Filter returns a new slice containing only the elements of in
// for which fn returns true.
//
// Type Parameters:
//   - T: the type of elements in the input slice
func Filter[T any](fn func(T) bool, in []T) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if fn(v) {
			out = append(out, v)
		}
	}
	return out
}

// Reduce applies fn cumulatively to the elements of in, from left to right,
// so as to reduce the slice to a single value. The zero value of T is used
// as the initial accumulator.
//
// Type Parameters:
//   - T: the type of elements in the input slice
func Reduce[T any](fn func(T, T) T, in []T) T {
	var result T
	for _, v := range in {
		result = fn(result, v)
	}
	return result
}

// ReduceWith applies the reduce operation on the input list with the
// zero initialized value and then apply the input value.
func ReduceWith[T any](fn func(T, T) T, in []T, init T) T {
	return Reduce(fn, append([]T{init}, in...))
}

type Tuple[T, U any] struct {
	First  T // First element of the tuple
	Second U // Second element of the tuple
}

// Enumerate returns a slice of Tuple[int, T], where each element contains
// the index and the corresponding value from the input slice `in`.
//
// It is the Go equivalent of Python's enumerate() function.
//
// Type Parameters:
//   - T: the type of elements in the input slice
func Enumerate[T any](in []T) []Tuple[int, T] {
	indexed := make([]Tuple[int, T], len(in))
	for idx, v := range in {
		indexed[idx] = Tuple[int, T]{idx, v}
	}
	return indexed
}

// ApplyToSecond takes a function `fn` that operates on a value of type T
// and returns a new function that operates on a Tuple[V, T], applying `fn`
// only to the Second element of the tuple.
//
// This is useful for mapping or transforming tuples while keeping the first
// element unchanged.
//
// Type Parameters:
//   - V: the type of the first element in the tuple
//   - T: the type of the second element in the tuple, which is the input type for `fn`
//   - U: the output type of `fn` and the returned function
func ApplyToSecond[V, T, U any](fn func(T) U) func(Tuple[V, T]) U {
	return func(t Tuple[V, T]) U {
		return fn(t.Second)
	}
}

// ApplyToFirst takes a function `fn` that operates on a value of type T
// and returns a new function that operates on a Tuple[V, T], applying `fn`
// only to the First element of the tuple.
//
// This is useful for mapping or transforming tuples while keeping the second
// element unchanged.
//
// Type Parameters:
//   - V: the type of the first element in the tuple, which is the input type for `fn`
//   - T: the type of the second element in the tuple
//   - U: the output type of `fn` and the returned function
func ApplyToFirst[V, T, U any](fn func(V) U) func(Tuple[V, T]) U {
	return func(t Tuple[V, T]) U {
		return fn(t.First)
	}
}

// And returns the logical and between the input values
func And(a, b bool) bool {
	return a && b
}

// Not returns the logical not of the input parameter
func Not(a bool) bool {
	return !a
}
