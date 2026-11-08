with open('test.txt', 'r', encoding='UTF-8') as f:
    data = f.read().split('\n')
    with open('anek_data.txt', 'w', encoding='UTF-8') as o:
        for line in data:
            new_line = line.replace("'", '')
            o.write(f'{new_line}\n')
