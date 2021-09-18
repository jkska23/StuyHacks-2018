from websocket import create_connection
from termcolor import cprint
from colorama import Fore, Back, Style
import json
import time


ws = create_connection("ws://localhost:8080")
try:
    print("You have connected to the CAH server")
    print("Here are the available games:")
    games = json.loads(ws.recv())
    for i, game in enumerate(games, start=1):
        print("{}. {}".format(i, game))
    choice = input("Write a number to choose a game, or write something else to start one - ")
    if choice.isdigit():
        while (not choice.isdigit()) or int(choice) < 1 or int(choice) > len(games): 
            choice = input("Invalid option. Write a number to choose a game - ")
        ws.send(games[int(choice) - 1])
    else:
        ws.send(choice)

    ws.send(input("Enter your username: "))
    print("Hand:")
    hand = json.loads(ws.recv())
    for i, card in enumerate(hand, 1):
        print("{}. {}".format(i, card))

    while True:
        message = ws.recv()
        print(Fore.BLACK + Back.WHITE + ws.recv() + Style.RESET_ALL) #black card
        if message == "judge":
            print(Fore.GREEN + "You're judging" + Style.RESET_ALL)
            no = 0 
            msg1 = ws.recv()
            while msg1 != "finished":
                no = no + 1
                print(Fore.WHITE + Back.BLACK + "{}. {}".format(no, msg1) + Style.RESET_ALL) # white card
                msg1 = ws.recv()
            
            choice = input("Select a card [1-{}]: \n ".format(no))
            while (not choice.isdigit()) or int(choice) < 0 or int(choice) > no:
                print("Invalid choice")
                choice = input("Select a card [1-{}]: \n ".format(no))

            ws.send(choice)
        elif message == "play":
            print(Fore.GREEN + "You're playing" + Style.RESET_ALL)
            hand = json.loads(ws.recv()) #receive message # hand 

            for i, card in enumerate(hand, start=1):
                print(Fore.WHITE + Back.BLACK + "{}. {}".format(i, card) + Style.RESET_ALL) #white cards

            choice = input("Select a card [1-{}]: \n ".format(len(hand)))
            while (not choice.isdigit()) or int(choice) < 0 or int(choice) > len(hand):
                print("Invalid choice")
                choice = input("Select a card [1-{}]: \n ".format(len(hand)))
            ws.send(choice)
            #show hand and ask user which one they want
            #send their choice
            print(Fore.BLUE + Back.YELLOW + ws.recv() + " wins with..." + Style.RESET_ALL) #shows winner
            print(Fore.BLUE + Back.YELLOW + ws.recv() + "!" + Style.RESET_ALL) #winning card 
        elif message == "dup":
            exit()
        else:
            print(message)

        print("Next round starting in...")
        n = int(3)
        while n>0:
            print(n)
            time.sleep(1)
            n = n - 1
except KeyboardInterrupt:
    ws.close()
