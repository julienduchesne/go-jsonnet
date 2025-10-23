// Demonstration of hermetic function caching
//
// Hermetic functions (functions with no external references) are automatically
// cached when called with evaluated arguments. This can significantly improve
// performance when the same function is called multiple times with the same arguments.
//
// The cache is global and shared across all evaluations and threads.

local expensiveComputation(n) =
  // This function has no external references, so it's hermetic
  if n <= 1 then
    n
  else
    expensiveComputation(n - 1) + expensiveComputation(n - 2);

{
  // These calls use tailstrict which forces argument evaluation
  // This enables caching for hermetic functions
  fib_10_first: expensiveComputation(10) tailstrict,  // Cache miss - computed
  fib_10_second: expensiveComputation(10) tailstrict,  // Cache hit - instant!
  fib_10_third: expensiveComputation(10) tailstrict,  // Cache hit - instant!

  fib_15_first: expensiveComputation(15) tailstrict,  // Cache miss - computed
  fib_15_second: expensiveComputation(15) tailstrict,  // Cache hit - instant!

  // Note: Without tailstrict, arguments may not be evaluated yet,
  // so caching cannot be used (to preserve lazy evaluation semantics)
  fib_10_lazy: expensiveComputation(10),  // Not cached
}
