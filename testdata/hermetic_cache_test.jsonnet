local expensiveFunction(x) = x * x;
{
	a: std.assertEqual(expensiveFunction(5) tailstrict, 25),
	b: std.assertEqual(expensiveFunction(5) tailstrict, 25),  // Should hit cache
	c: std.assertEqual(expensiveFunction(10) tailstrict, 100),
	d: std.assertEqual(expensiveFunction(10) tailstrict, 100), // Should hit cache
	result: true
}

