i=1
sum=0
while true; do
  test $i -gt 10 && break
  sum=$(expr $sum + $i)
  echo index $i sum $sum
  i=$(expr $i + 1)
done
