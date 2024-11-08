[ -z $END ] && echo '$END variable not exported' && exit 1
for i in $(seq 0 $END); do
  echo "Release the krkn $i"
  sleep 1
done