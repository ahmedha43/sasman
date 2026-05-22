import npkpy
import inspect

print("Available in npkpy module:")
print(dir(npkpy))

# Check signature
for name, obj in inspect.getmembers(npkpy):
    if not name.startswith('_'):
        print(f"{name}: {type(obj)}")