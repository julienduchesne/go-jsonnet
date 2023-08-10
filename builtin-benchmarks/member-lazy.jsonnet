{
  getValue:: function(i) '%s - formatting is expensive!' % [std.toString(i)],

  foo: [
    i
    for i in std.range(0, 100)
    for y in std.range(0, 100)
    // std.getValue shouldn't be called since the array is empty
    if std.member([], self.getValue(i))
  ],
}
