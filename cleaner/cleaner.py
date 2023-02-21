with open('aneks.txt', 'r', encoding='UTF-8') as f:
    data = f.read().split('\n')
    with open('no_insert_aneks.txt', 'w', encoding='UTF-8') as o:
        for line in data:
            start = line.find(',')
            stop = line.find(')')
            o.write(f'{line[start+1:stop]}\n')
