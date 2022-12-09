$(document).ready( function() {

    webcamImg = document.getElementById('webcam');
    logConsole = document.getElementById('console');
    
    const socket = io.connect(':9099', { transports: ['websocket'] });
    var step = 10;
    $( "#step10" ).prop( "disabled", true );


    function addToConsole(text) {
        logConsole.innerHTML += '\n' + text;
    }

    socket.on("log",function(data) {
        addToConsole(data.text);
    });

    socket.on("webcam",function(data) {
        webcamImg.src = 'data:image/png;base64,' + data.text;
    });


    $('#xyup').on('click', function (e) {
        console.log("Move up")
        socket.emit('printer', { text: 'move', body: { axis: 'Y', mm: step } }, function(response) {
            console.log(response);
        });
    })

    $('#xydown').on('click', function (e) {
        console.log("Move down")
        socket.emit('printer', { text: 'move', body: { axis: 'Y', mm: (step * -1) } }, function(response) {
            console.log(response);
        });
    })


    $('#xyleft').on('click', function (e) {
        console.log("Move left")
        socket.emit('printer', { text: 'move', body: { axis: 'X', mm: (step * -1) } }, function(response) {
            console.log(response);
        });
    })

    $('#xyright').on('click', function (e) {
        console.log("Move right")
        socket.emit('printer', { text: 'move', body: { axis: 'X', mm: step } }, function(response) {
            console.log(response);
        });
    })


    $('#zup').on('click', function (e) {
        console.log("Move right")
        socket.emit('printer', { text: 'move', body: { axis: 'Z', mm: step } }, function(response) {
            console.log(response);
        });
    })

    $('#zdown').on('click', function (e) {
        console.log("Move right")
        socket.emit('printer', { text: 'move', body: { axis: 'Z', mm: (step * -1) } }, function(response) {
            console.log(response);
        });
    })


    $('#zhome').on('click', function (e) {
        console.log("Home Z")
        socket.emit('printer', { text: 'homez' }, function(response) {
            console.log(response);
        });
    })

    $('#xyhome').on('click', function (e) {
        console.log("Home XY")
        socket.emit('printer', { text: 'homexy' }, function(response) {
            console.log(response);
        });
    })


    $('#btnstart').on('click', function (e) {
        console.log("Starting capture")

        const sendData = {
            text: 'start',
            body: {
                width: parseInt($('#pcbwidth').val()),
                height: parseInt($('#pcbheight').val()),
                step: parseInt($('#steps').val()),
            }
        };

        console.log(sendData);
        
        socket.emit('panorama', sendData, function(response) {
            console.log(response);
        });
    })


    $('#btnstop').on('click', function (e) {
        console.log("Stop capture")
        
        socket.emit('panorama', { text: 'stop' }, function(response) {
            console.log(response);
        });
    })


    $('#download').on('click', function (e) {
        console.log("Download results")
        
        window.location="results.zip";

    })


    function changeSteps(newStep) {

        $( "#step1" ).prop( "disabled", false );
        $( "#step10" ).prop( "disabled", false );
        $( "#step100" ).prop( "disabled", false );

        $( "#step" + newStep ).prop( "disabled", true );

        step = newStep;
    }


    $('#step1').on('click', function (e) {
        changeSteps(1)
    })

    $('#step10').on('click', function (e) {
        changeSteps(10)
    })

    $('#step100').on('click', function (e) {
        changeSteps(100)
    })

});
