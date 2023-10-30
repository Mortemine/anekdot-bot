import requests
import telebot
from bs4 import BeautifulSoup
from telebot import types
import database.model as model


token = ''
bot = telebot.TeleBot(token, threaded=False)
url = 'https://baneks.site/random'
previous_anek = {}
model.start_session()

print('Database successfully loaded')


def get_random_anek(message):
    response = requests.get(url)
    soup = BeautifulSoup(response.text, 'html.parser')
    for e in soup.findAll('br'):
        e.replaceWith('\n')
    anek = soup.find('p').get_text('')
    keyboard = types.InlineKeyboardMarkup()
    next_button = types.InlineKeyboardButton('Следующий!', callback_data='next')
    keyboard.add(next_button)
    previous_anek[message.from_user.id] = bot.send_message(message.from_user.id, anek, reply_markup=keyboard).message_id


def send_to_admin(message):
    if message.text == 'Назад':
        to_start(message)
    else:
        bot.send_message(409197843, f'Поступил запрос от @{message.from_user.username} из чата №{message.chat.id}'
                                    f' с сообщением:\n'
                                    f'{message.text}')
        bot.send_message(message.chat.id, 'Ваш запрос был отправлен администрации!')


def make_buttons(message, button_values, message_text):
    markup = types.ReplyKeyboardMarkup(resize_keyboard=True)
    for value in button_values:
        markup.add(types.KeyboardButton(value))
    back_button = types.KeyboardButton('Назад')
    markup.add(back_button)
    bot.send_message(message.chat.id, message_text, reply_markup=markup)


def start_dialog_buttons():
    markup = types.ReplyKeyboardMarkup(resize_keyboard=True)
    categories_button = types.KeyboardButton('Анекдоты по категориям')
    random_button = types.KeyboardButton('Случайный анекдот')
    admin_button = types.KeyboardButton('Связь с администрацией')
    markup.add(categories_button, random_button, admin_button)
    return markup


def to_start(message):
    markup = start_dialog_buttons()
    bot.send_message(message.chat.id, 'Вы вернулись в начало', reply_markup=markup)


@bot.message_handler(commands=['start'])
def dialog_start(message):
    markup = start_dialog_buttons()
    bot.send_message(message.chat.id, 'Это бот с анекдотами. Что он умеет:\n'
                                      '\n'
                                      'Отправлять анекдоты по категориям;\n'
                                      'Отправлять слуайный анекдот;\n'
                                      'А так же вы можете связаться с администрацией по любому вопросу.\n'
                                      '\n'
                                      'Всё взаимодействие с ботом происходит через панель снизу.\n'
                                      '\n'
                                      'БУДЬТЕ ОСТОРОЖНЫ! Анекдоты зачастую нецензурные.',
                     reply_markup=markup)


@bot.callback_query_handler(lambda call: True)
def callback_handler(call):
    if call.data == 'next':
        try:
            bot.delete_message(call.from_user.id, previous_anek[call.from_user.id])
        except BaseException:
            pass
        get_random_anek(call)


@bot.message_handler(content_types=['text'])
def text_message(message):
    markup = types.ReplyKeyboardMarkup(resize_keyboard=True)
    back_button = types.KeyboardButton('Назад')
    categories = model.get_all_categories()
    if message.text == "Анекдоты по категориям":
        make_buttons(message, categories, 'Выберите категорию')
    elif message.text == 'Случайный анекдот':
        get_random_anek(message)
    elif message.text == 'Связь с администрацией':
        markup.add(back_button)
        bot.send_message(message.chat.id, 'Напишите сообщение и оно направится администрации.', reply_markup=markup)
        bot.register_next_step_handler(message, send_to_admin)
    elif message.text == 'Назад':
        to_start(message)
    elif message.text in categories:
        bot.send_message(message.chat.id, model.get_random_anek(message.text))


while True:
    try:
        bot.polling()
    except BaseException as error:
        print(error.__class__)
